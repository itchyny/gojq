package gojq

import (
	"cmp"
	"context"
	"math"
	"reflect"
	"slices"
)

func (env *env) execute(bc *Code, v any, vars ...any) Iter {
	if len(vars)+1 > env.maxStackDepth {
		return NewIter(&stackLimitError{})
	}
	env.codes = bc.codes
	env.codeinfos = bc.codeinfos
	env.push(v)
	for i := len(vars) - 1; i >= 0; i-- {
		env.push(vars[i])
	}
	env.debugCodes()
	return env
}

func (env *env) Next() (v any, ok bool) {
	var err error
	pc, callpc, index := env.pc, len(env.codes)-1, -1
	backtrack, hasCtx := env.backtrack, env.ctx != context.Background()
	defer func() {
		// A few value operations recurse in Go outside the opcode loop and panic
		// with *allocLimitError when they get too deep or too large (Compare over a
		// deeply nested value, for one). Interpreter stack pushes use the same
		// internal unwind for *stackLimitError. Recover both here so they surface as
		// typed errors instead of escaping the run; anything else re-panics.
		if r := recover(); r != nil {
			switch r.(type) {
			case *allocLimitError, *stackLimitError:
			default:
				panic(r)
			}
			pc, env.forks = len(env.codes), nil
			v, ok = r, true
		}
		env.pc, env.backtrack = pc, true
	}()
loop:
	for ; pc < len(env.codes); pc++ {
		env.debugState(pc, backtrack)
		code := env.codes[pc]
		if hasCtx {
			select {
			case <-env.ctx.Done():
				pc, env.forks = len(env.codes), nil
				return env.ctx.Err(), true
			default:
			}
		}
		if MaxAlloc > 0 && env.overStackLimit() {
			pc, env.forks = len(env.codes), nil
			return &allocLimitError{}, true
		}
		switch code.op {
		case opnop:
			// nop
		case oppush:
			env.push(code.v)
		case oppop:
			env.pop()
		case opdup:
			v := env.pop()
			env.push(v)
			env.push(v)
		case opconst:
			env.pop()
			env.push(code.v)
		case opload:
			env.push(env.values[env.index(code.v.([2]int))])
		case opstore:
			env.values[env.index(code.v.([2]int))] = env.pop()
		case opobject:
			if backtrack {
				break loop
			}
			n := code.v.(int)
			m := make(map[string]any, n)
			for range n {
				v, k := env.pop(), env.pop()
				s, ok := k.(string)
				if !ok {
					err = &objectKeyNotStringError{k}
					break loop
				}
				if _, ok := m[s]; !ok {
					m[s] = v
				}
			}
			env.push(m)
			if env.charge(m) {
				err = &allocLimitError{}
				break loop
			}
		case opappend:
			i := env.index(code.v.([2]int))
			x := env.pop()
			xs := env.values[i].([]any)
			// append can hold both old and new 16-byte-slot backing arrays during
			// geometric growth. Bound that peak before it can make a large array
			// whose transient footprint already exceeds the run limit.
			if arrayAppendTooLarge(len(xs) + 1) {
				err = &allocLimitError{}
				break loop
			}
			if env.chargeBytes(16 + allocSize(x)) {
				err = &allocLimitError{}
				break loop
			}
			env.values[i] = append(xs, x)
		case opfork:
			if backtrack {
				if err != nil {
					break loop
				}
				pc, backtrack = code.v.(int), false
				goto loop
			}
			env.pushfork(pc)
		case opforktrybegin:
			if backtrack {
				switch e := err.(type) {
				case *tryEndError:
					err = e.err
					break loop
				case nil, *breakError, *HaltError:
					break loop
				case ValueError:
					env.pop()
					env.push(e.Value())
				default:
					env.pop()
					env.push(err.Error())
				}
				pc, backtrack, err = code.v.(int), false, nil
				goto loop
			}
			env.pushfork(pc)
		case opforktryend:
			if backtrack {
				if err != nil {
					err = &tryEndError{err}
				}
				break loop
			}
			env.pushfork(pc)
		case opforkalt:
			if backtrack {
				switch err.(type) {
				case nil, *breakError, *HaltError:
					break loop
				}
				pc, backtrack, err = code.v.(int), false, nil
				goto loop
			}
			env.pushfork(pc)
		case opforklabel:
			if backtrack {
				label := env.pop()
				if e, ok := err.(*breakError); ok && e.v == label {
					err = nil
				}
				break loop
			}
			env.push(env.label)
			env.pushfork(pc)
			env.pop()
			env.values[env.index(code.v.([2]int))] = env.label
			env.label++
		case opbacktrack:
			break loop
		case opjump:
			pc = code.v.(int)
			goto loop
		case opjumpifnot:
			if v := env.pop(); v == nil || v == false {
				pc = code.v.(int)
				goto loop
			}
		case opindex, opindexarray:
			if backtrack {
				break loop
			}
			p, v := code.v, env.pop()
			if code.op == opindexarray && v != nil {
				if _, ok := v.([]any); !ok {
					err = &expectedArrayError{v}
					break loop
				}
			}
			w := funcIndex2(nil, v, p)
			if e, ok := w.(error); ok {
				err = e
				break loop
			}
			env.push(w)
			if !env.paths.empty() && env.expdepth == 0 {
				if !env.pathIntact(v) {
					err = &invalidPathError{v}
					break loop
				}
				env.pushpath(pathValue{path: p, value: w})
			}
		case opcall:
			if backtrack {
				break loop
			}
			switch v := code.v.(type) {
			case int:
				pc, callpc, index = v, pc, env.scopes.index
				goto loop
			case [3]any:
				argcnt := v[1].(int)
				x, args := env.pop(), env.args[:argcnt]
				for i := range argcnt {
					args[i] = env.pop()
				}
				w := v[0].(func(any, []any) any)(x, args)
				if e, ok := w.(error); ok {
					err = e
					break loop
				}
				env.push(w)
				exceeded := false
				if MaxAlloc > 0 {
					switch w.(type) {
					case nil, bool, int, float64:
						// The scalar hot path allocates nothing. Keep only the
						// monotonic-budget check needed after a caught limit error.
						exceeded = env.alloc > MaxAlloc
					default:
						n := allocSize(w)
						exceeded = env.alloc > MaxAlloc
						if n > 0 {
							name, _ := v[2].(string)
							if name != "input" && name != "inputs" {
								switch w.(type) {
								case []any, map[string]any:
									exceeded = env.chargeFreshResult(w, x, args)
								default:
									exceeded = env.chargeBytes(n)
								}
							}
						}
					}
				}
				if exceeded {
					err = &allocLimitError{}
					break loop
				}
				if !env.paths.empty() && env.expdepth == 0 {
					switch v[2].(string) {
					case "_index":
						if x = args[0]; !env.pathIntact(x) {
							err = &invalidPathError{x}
							break loop
						}
						env.pushpath(pathValue{path: args[1], value: w})
					case "_slice":
						if x = args[0]; !env.pathIntact(x) {
							err = &invalidPathError{x}
							break loop
						}
						env.pushpath(pathValue{
							path:  map[string]any{"start": args[2], "end": args[1]},
							value: w,
						})
					case "getpath":
						if !env.pathIntact(x) {
							err = &invalidPathError{x}
							break loop
						}
						for _, p := range args[0].([]any) {
							env.pushpath(pathValue{path: p, value: w})
						}
					}
				}
			default:
				panic(v)
			}
		case opcallrec:
			pc, callpc, index = code.v.(int), -1, env.scopes.index
			goto loop
		case opcalltail:
			env.tailDepth++
			env.checkStackDepth(env.tailDepth + max(env.scopes.index, env.scopes.limit) + 1)
			pc, callpc, index = code.v.(int), -1, env.scopes.index
			goto loop
		case oppushpc:
			env.push([2]int{code.v.(int), env.scopes.index})
		case opcallpc:
			xs := env.pop().([2]int)
			pc, callpc, index = xs[0], pc, xs[1]
			goto loop
		case opscope:
			xs := code.v.(scopeCode)
			var saveindex, outerindex int
			tailDepth := env.tailDepth
			if index == env.scopes.index {
				if callpc >= 0 {
					saveindex = index
				} else {
					s := env.popscope()
					callpc, saveindex, tailDepth = s.pc, s.saveindex, s.tailDepth
				}
			} else {
				saveindex = env.scopes.index
			}
			if outerindex = index; outerindex >= 0 {
				if s := env.scopes.data[outerindex].value; s.id == xs.id {
					outerindex = s.outerindex
				}
			}
			env.pushscope(scope{
				id: xs.id, offset: env.offset, pc: callpc,
				saveindex: saveindex, outerindex: outerindex, tailDepth: tailDepth,
			})
			env.checkStackDepth(env.offset + xs.variableCount)
			env.offset += xs.variableCount
			if env.offset > len(env.values) {
				vs := make([]any, env.offset*2)
				copy(vs, env.values)
				env.values = vs
			}
		case opret:
			if backtrack {
				break loop
			}
			s := env.popscope()
			pc, env.scopes.index, env.tailDepth = s.pc, s.saveindex, s.tailDepth
			if env.scopes.empty() {
				return env.pop(), true
			}
		case opiter:
			if err != nil {
				break loop
			}
			backtrack = false
			var xs []pathValue
			switch v := env.pop().(type) {
			case []pathValue:
				xs = v
			case []any:
				if !env.paths.empty() && env.expdepth == 0 && !env.pathIntact(v) {
					err = &invalidPathIterError{v}
					env.push(emptyIter{})
					break loop
				}
				if len(v) == 0 {
					break loop
				}
				// .[] materializes a []pathValue of every element ( 32 bytes each )
				// to drive the iteration. A per-array pre-check bounded one array
				// but never charged it, so nested iteration of the same large array
				// ( $a[] as $x | $a[] as $y | ... ) stacked these uncharged and
				// reached hundreds of MB. Charge it to the run meter so nested and
				// looped iterations accumulate and trip.
				if env.chargeBytes(int64(len(v)) * 32) {
					err = &allocLimitError{}
					break loop
				}
				xs = make([]pathValue, len(v))
				for i, v := range v {
					xs[i] = pathValue{path: i, value: v}
				}
			case map[string]any:
				if !env.paths.empty() && env.expdepth == 0 && !env.pathIntact(v) {
					err = &invalidPathIterError{v}
					env.push(emptyIter{})
					break loop
				}
				if len(v) == 0 {
					break loop
				}
				// .[] materializes a []pathValue of every element ( 32 bytes each )
				// to drive the iteration. A per-array pre-check bounded one array
				// but never charged it, so nested iteration of the same large array
				// ( $a[] as $x | $a[] as $y | ... ) stacked these uncharged and
				// reached hundreds of MB. Charge it to the run meter so nested and
				// looped iterations accumulate and trip.
				if env.chargeBytes(int64(len(v)) * 32) {
					err = &allocLimitError{}
					break loop
				}
				xs = make([]pathValue, len(v))
				var i int
				for k, v := range v {
					xs[i] = pathValue{path: k, value: v}
					i++
				}
				slices.SortFunc(xs, func(x, y pathValue) int {
					return cmp.Compare(x.path.(string), y.path.(string))
				})
			case Iter:
				if w, ok := v.Next(); ok {
					env.push(v)
					env.pushfork(pc)
					env.pop()
					if e, ok := w.(error); ok {
						err = e
						break loop
					}
					exceeded := false
					if MaxAlloc > 0 {
						switch w.(type) {
						case nil, bool, int, float64:
							exceeded = env.alloc > MaxAlloc
						default:
							n := allocSize(w)
							exceeded = env.alloc > MaxAlloc
							if n > 0 {
								switch w.(type) {
								case []any, map[string]any:
									exceeded = env.chargeFreshResult(w, nil, nil)
								default:
									exceeded = env.chargeBytes(n)
								}
							}
						}
					}
					if exceeded {
						err = &allocLimitError{}
						break loop
					}
					env.push(w)
					continue
				}
				break loop
			default:
				err = &iteratorError{v}
				env.push(emptyIter{})
				break loop
			}
			if len(xs) > 1 {
				env.push(xs[1:])
				env.pushfork(pc)
				env.pop()
			}
			env.push(xs[0].value)
			if !env.paths.empty() && env.expdepth == 0 {
				env.pushpath(xs[0])
			}
		case opexpbegin:
			env.expdepth++
		case opexpend:
			env.expdepth--
		case oppathbegin:
			env.pushpath(env.expdepth)
			env.pushpath(pathValue{value: env.stack.top()})
			env.expdepth = 0
		case oppathend:
			if backtrack {
				break loop
			}
			env.pop()
			if v := env.pop(); !env.pathIntact(v) {
				err = &invalidPathError{v}
				break loop
			}
			paths := env.poppaths()
			if env.charge(paths) {
				err = &allocLimitError{}
				break loop
			}
			env.push(paths)
			env.expdepth = env.poppath().(int)
		default:
			panic(code.op)
		}
	}
	if len(env.forks) > 0 {
		pc, backtrack = env.popfork(), true
		goto loop
	}
	if err != nil {
		return err, true
	}
	return nil, false
}

func (env *env) push(v any) {
	env.stack.push(v, env.maxStackDepth)
}

func (env *env) pop() any {
	return env.stack.pop()
}

func (env *env) pushpath(v any) {
	env.paths.push(v, env.maxStackDepth)
}

func (env *env) poppath() any {
	return env.paths.pop()
}

func (env *env) pushscope(s scope) {
	env.checkStackDepth(nextScopeDepth(env.scopes) + env.tailDepth)
	env.scopes.push(s)
}

func (env *env) popscope() scope {
	free := env.scopes.index > env.scopes.limit
	s := env.scopes.pop()
	if free {
		env.offset = s.offset
	}
	return s
}

func (env *env) pushfork(pc int) {
	env.checkStackDepth(len(env.forks) + 1)
	f := fork{
		pc: pc, offset: env.offset, expdepth: env.expdepth,
		tailDepth: env.tailDepth,
	}
	f.stackindex, f.stacklimit = env.stack.save()
	f.scopeindex, f.scopelimit = env.scopes.save()
	f.pathindex, f.pathlimit = env.paths.save()
	env.forks = append(env.forks, f)
	env.debugForks(pc, ">>>")
}

func (env *env) popfork() int {
	f := env.forks[len(env.forks)-1]
	env.debugForks(f.pc, "<<<")
	env.forks, env.offset, env.expdepth, env.tailDepth =
		env.forks[:len(env.forks)-1], f.offset, f.expdepth, f.tailDepth
	env.stack.restore(f.stackindex, f.stacklimit)
	env.scopes.restore(f.scopeindex, f.scopelimit)
	env.paths.restore(f.pathindex, f.pathlimit)
	return f.pc
}

func (env *env) index(v [2]int) int {
	for id, i := v[0], env.scopes.index; i >= 0; {
		s := env.scopes.data[i].value
		if s.id == id {
			return s.offset + v[1]
		}
		i = s.outerindex
	}
	panic("env.index")
}

type pathValue struct {
	path, value any
}

func (env *env) pathIntact(v any) bool {
	w := env.paths.top().(pathValue).value
	switch v := v.(type) {
	case []any, map[string]any:
		switch w.(type) {
		case []any, map[string]any:
			v, w := reflect.ValueOf(v), reflect.ValueOf(w)
			return v.Pointer() == w.Pointer() && v.Len() == w.Len()
		}
	case float64:
		if w, ok := w.(float64); ok {
			return v == w || math.IsNaN(v) && math.IsNaN(w)
		}
	}
	return v == w
}

func (env *env) poppaths() []any {
	xs := []any{}
	for {
		p := env.poppath().(pathValue)
		if p.path == nil {
			break
		}
		xs = append(xs, p.path)
	}
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
	return xs
}
