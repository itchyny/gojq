package gojq

func (env *env) execute(bc *bytecode, v interface{}) Iter {
	env.codes = bc.codes
	env.codeinfos = bc.codeinfos
	env.push(v)
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	pc, callpc, err := env.pc, 0, error(nil)
loop:
	for ; 0 <= pc && pc < len(env.codes); pc++ {
		env.debugState(pc)
		code := env.codes[pc]
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
		case opswap:
			x, y := env.pop(), env.pop()
			env.push(x)
			env.push(y)
		case opconst:
			env.pop()
			env.push(code.v)
		case opload:
			env.push(env.value[env.index(code.v.([2]int))])
		case opstore:
			i := env.index(code.v.([2]int))
			if i >= len(env.value) {
				vs := make([]interface{}, (i+1)*2)
				copy(vs, env.value)
				env.value = vs
			}
			env.value[i] = env.pop()
		case opappend:
			i := env.index(code.v.([2]int))
			env.value[i] = append(env.value[i].([]interface{}), env.pop())
		case opfork:
			env.pushfork(code.op, code.v.(int))
		case opbacktrack:
			if len(env.forks) > 0 {
				pc++
				break loop
			}
			return nil, false
		case opjump:
			pc = code.v.(int)
		case opjumppop:
			pc, callpc = env.pop().(int), pc
		case opjumpifnot:
			if !valueToBool(env.pop()) {
				pc = code.v.(int)
			}
		case opret:
			pc = env.scopes.pop().(scope).pc
			if env.scopes.empty() {
				env.pc = len(env.codes)
				if env.stack.empty() {
					return nil, false
				}
				return env.pop(), true
			}
		case opcall:
			switch v := code.v.(type) {
			case int:
				pc, callpc = v, pc
			case [3]interface{}:
				argcnt := v[1].(int)
				x, args := env.pop(), make([]interface{}, argcnt)
				for i := 0; i < argcnt; i++ {
					args[i] = env.pop()
				}
				w := v[0].(func(interface{}, []interface{}) interface{})(x, args)
				if e, ok := w.(error); ok {
					err = e
					goto on_err
				}
				env.push(w)
			default:
				panic(v)
			}
		case opscope:
			xs := code.v.([2]int)
			offset := -1
			if !env.scopes.empty() {
				offset = env.scopes.top().(scope).offset
			}
			env.scopes.push(scope{xs[0], offset + xs[1], callpc})
		case opeach:
			switch v := env.pop().(type) {
			case []interface{}:
				if len(v) > 0 {
					if len(v) > 1 {
						env.push(v[1:])
						env.pushfork(code.op, pc)
						env.pop()
					}
					env.push(v[0])
					pc++
				}
			case map[string]interface{}:
				a := make([]interface{}, len(v))
				var i int
				for _, v := range v {
					a[i] = v
					i++
				}
				if len(a) > 0 {
					if len(v) > 1 {
						env.push(a[1:])
						env.pushfork(code.op, pc)
						env.pop()
					}
					env.push(a[0])
					pc++
				}
			default:
				err = &iteratorError{v}
				goto on_err
			}
		default:
			panic(code.op)
		}
	}
	env.pc = pc
	if len(env.forks) > 0 {
		f := env.popfork()
		pc = f.pc
		goto loop
	}
	return nil, false
on_err:
	for !env.scopes.empty() {
		pc = env.scopes.pop().(scope).pc
	}
	env.pc = pc
	return err, true
}

func (env *env) push(v interface{}) {
	env.stack.push(v)
}

func (env *env) pop() interface{} {
	return env.stack.pop()
}

func (env *env) pushfork(op opcode, pc int) {
	f := &fork{op: op, pc: pc}
	env.stack.save(&f.stackindex, &f.stacklimit)
	env.scopes.save(&f.scopeindex, &f.scopelimit)
	env.forks = append(env.forks, f)
	env.debugForks(pc, ">>>")
}

func (env *env) popfork() *fork {
	f := env.forks[len(env.forks)-1]
	env.debugForks(f.pc, "<<<")
	env.forks = env.forks[:len(env.forks)-1]
	env.stack.restore(f.stackindex, f.stacklimit)
	env.scopes.restore(f.scopeindex, f.scopelimit)
	return f
}

func (env *env) scopeOffset(id int) int {
	return env.scopes.lookup(func(v interface{}) bool {
		return v.(scope).id == id
	}).(scope).offset
}

func (env *env) index(v [2]int) int {
	return env.scopeOffset(v[0]) - v[1]
}
