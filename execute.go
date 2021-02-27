package gojq

import (
	"fmt"
	"reflect"
	"sort"
)

func (env *env) execute(bc *Code, v interface{}, vars ...interface{}) Iter {
	env.codes = bc.codes
	env.codeinfos = bc.codeinfos
	env.push(v)
	for i := len(vars) - 1; i >= 0; i-- {
		env.push(vars[i])
	}
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	var err error
	pc, callpc, index := env.pc, len(env.codes)-1, -1
	backtrack, hasCtx := env.backtrack, env.ctx != nil
	defer func() { env.pc, env.backtrack = pc, true }()
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
		switch code.op {
		case opnop:
			// nop
		case oppush:
			env.push(code.v)
		case oppop:
			env.pop()
		case opdup:
			x := env.pop()
			env.push(x)
			env.push(x)
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
			m := make(map[string]interface{}, n)
			for i := 0; i < n; i++ {
				v, k := env.pop(), env.pop()
				s, ok := k.(string)
				if !ok {
					err = &objectKeyNotStringError{k}
					break loop
				}
				m[s] = v
			}
			env.push(m)
		case opappend:
			i := env.index(code.v.([2]int))
			env.values[i] = append(env.values[i].([]interface{}), env.pop())
		case opfork:
			if backtrack {
				if err != nil {
					break loop
				}
				pc, backtrack = code.v.(int), false
				goto loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opforktrybegin:
			if backtrack {
				if err == nil {
					break loop
				}
				switch er := err.(type) {
				case *tryEndError:
					err = er.err
					break loop
				case ValueError:
					if er, ok := er.(*exitCodeError); ok && er.halt {
						break loop
					}
					if v := er.Value(); v != nil {
						env.pop()
						env.push(v)
					} else {
						err = nil
						break loop
					}
				default:
					env.pop()
					env.push(err.Error())
				}
				pc, backtrack, err = code.v.(int), false, nil
				goto loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opforktryend:
			if backtrack {
				if err != nil {
					err = &tryEndError{err}
				}
				break loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opforkalt:
			if backtrack {
				if err == nil {
					break loop
				}
				pc, backtrack, err = code.v.(int), false, nil
				goto loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opforklabel:
			if backtrack {
				if e, ok := err.(*breakError); ok && code.v.(string) == e.n {
					err = nil
				}
				break loop
			} else {
				env.pushfork(code.op, pc)
			}
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
		case opcall:
			if backtrack {
				break loop
			}
			switch v := code.v.(type) {
			case int:
				pc, callpc, index = v, pc, env.scopes.index
				goto loop
			case [3]interface{}:
				argcnt := v[1].(int)
				x, args := env.pop(), env.args[:argcnt]
				for i := 0; i < argcnt; i++ {
					args[i] = env.pop()
				}
				w := v[0].(func(interface{}, []interface{}) interface{})(x, args)
				if e, ok := w.(error); ok {
					err = e
					break loop
				}
				env.push(w)
				if !env.paths.empty() {
					var ps []interface{}
					ps, err = env.pathEntries(v[2].(string), x, args)
					if err != nil {
						break loop
					}
					for _, p := range ps {
						env.paths.push([2]interface{}{p, w})
					}
				}
			default:
				panic(v)
			}
		case oppushpc:
			env.push([2]int{code.v.(int), env.scopes.index})
		case opcallpc:
			xs := env.pop().([2]int)
			pc, callpc, index = xs[0], pc, xs[1]
			goto loop
		case opscope:
			xs := code.v.([2]int)
			var i, l int
			if index == env.scopes.index {
				i = index
			} else {
				env.scopes.save(&i, &l)
				env.scopes.index = index
			}
			env.scopes.push(scope{xs[0], env.offset, callpc, i})
			env.offset += xs[1]
			if env.offset > len(env.values) {
				vs := make([]interface{}, env.offset*2)
				copy(vs, env.values)
				env.values = vs
			}
		case opret:
			if backtrack {
				break loop
			}
			s := env.scopes.pop().(scope)
			pc, env.scopes.index = s.pc, s.saveindex
			if env.scopes.empty() {
				return env.pop(), true
			}
		case opeach:
			if err != nil {
				break loop
			}
			backtrack = false
			var xs [][2]interface{}
			switch v := env.pop().(type) {
			case [][2]interface{}:
				xs = v
			case []interface{}:
				if !env.paths.empty() && (env.expdepth == 0 && !reflect.DeepEqual(v, env.paths.top().([2]interface{})[1])) {
					err = &invalidPathIterError{v}
					break loop
				}
				if len(v) == 0 {
					break loop
				}
				xs = make([][2]interface{}, len(v))
				for i, v := range v {
					xs[i] = [2]interface{}{i, v}
				}
			case map[string]interface{}:
				if !env.paths.empty() && (env.expdepth == 0 && !reflect.DeepEqual(v, env.paths.top().([2]interface{})[1])) {
					err = &invalidPathIterError{v}
					break loop
				}
				if len(v) == 0 {
					break loop
				}
				xs = make([][2]interface{}, len(v))
				var i int
				for k, v := range v {
					xs[i] = [2]interface{}{k, v}
					i++
				}
				sort.Slice(xs, func(i, j int) bool {
					return xs[i][0].(string) < xs[j][0].(string)
				})
			default:
				err = &iteratorError{v}
				break loop
			}
			if len(xs) > 1 {
				env.push(xs[1:])
				env.pushfork(code.op, pc)
				env.pop()
			}
			env.push(xs[0][1])
			if !env.paths.empty() {
				if env.expdepth == 0 {
					env.paths.push(xs[0])
				}
			}
		case opexpbegin:
			env.expdepth++
		case opexpend:
			env.expdepth--
		case oppathbegin:
			env.paths.push(env.expdepth)
			env.paths.push([2]interface{}{nil, env.stack.top()})
			env.expdepth = 0
		case oppathend:
			if backtrack {
				break loop
			}
			if env.expdepth > 0 {
				panic(fmt.Sprintf("unexpected expdepth: %d", env.expdepth))
			}
			env.pop()
			x := env.pop()
			if reflect.DeepEqual(x, env.paths.top().([2]interface{})[1]) {
				env.push(env.poppaths())
				env.expdepth = env.paths.pop().(int)
			} else {
				err = &invalidPathError{x}
				break loop
			}
		case opdebug:
			if !backtrack {
				return [2]interface{}{code.v, env.stack.top()}, true
			}
			backtrack = false
		default:
			panic(code.op)
		}
	}
	if len(env.forks) > 0 {
		pc, backtrack = env.popfork().pc, true
		goto loop
	}
	if err != nil {
		return err, true
	}
	return nil, false
}

func (env *env) push(v interface{}) {
	env.stack.push(v)
}

func (env *env) pop() interface{} {
	return env.stack.pop()
}

func (env *env) pushfork(op opcode, pc int) {
	f := &fork{op: op, pc: pc, expdepth: env.expdepth}
	env.stack.save(&f.stackindex, &f.stacklimit)
	env.scopes.save(&f.scopeindex, &f.scopelimit)
	env.paths.save(&f.pathindex, &f.pathlimit)
	env.forks = append(env.forks, f)
	env.debugForks(pc, ">>>")
}

func (env *env) popfork() *fork {
	f := env.forks[len(env.forks)-1]
	env.debugForks(f.pc, "<<<")
	env.forks, env.expdepth = env.forks[:len(env.forks)-1], f.expdepth
	env.stack.restore(f.stackindex, f.stacklimit)
	env.scopes.restore(f.scopeindex, f.scopelimit)
	env.paths.restore(f.pathindex, f.pathlimit)
	return f
}

func (env *env) scopeOffset(id int) int {
	return env.scopes.lookup(func(v interface{}) bool {
		return v.(scope).id == id
	}).(scope).offset
}

func (env *env) index(v [2]int) int {
	return env.scopeOffset(v[0]) + v[1]
}

func (env *env) pathEntries(name string, x interface{}, args []interface{}) ([]interface{}, error) {
	switch name {
	case "_index":
		if env.expdepth > 0 {
			return nil, nil
		} else if !reflect.DeepEqual(args[0], env.paths.top().([2]interface{})[1]) {
			return nil, &invalidPathError{x}
		}
		return []interface{}{args[1]}, nil
	case "_slice":
		if env.expdepth > 0 {
			return nil, nil
		} else if !reflect.DeepEqual(args[0], env.paths.top().([2]interface{})[1]) {
			return nil, &invalidPathError{x}
		}
		return []interface{}{map[string]interface{}{"start": args[2], "end": args[1]}}, nil
	case "getpath":
		if env.expdepth > 0 {
			return nil, nil
		} else if !reflect.DeepEqual(x, env.paths.top().([2]interface{})[1]) {
			return nil, &invalidPathError{x}
		}
		return args[0].([]interface{}), nil
	default:
		return nil, nil
	}
}

func (env *env) poppaths() []interface{} {
	var xs []interface{}
	for {
		p := env.paths.pop().([2]interface{})
		if p[0] == nil {
			break
		}
		xs = append(xs, p[0])
	}
	for i := 0; i < len(xs)/2; i++ { // reverse
		j := len(xs) - 1 - i
		xs[i], xs[j] = xs[j], xs[i]
	}
	return xs
}
