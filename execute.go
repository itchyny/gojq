package gojq

func (env *env) execute(bc *bytecode, v interface{}) Iter {
	env.codes = bc.codes
	env.codeinfos = bc.codeinfos
	env.push(v)
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	pc := env.pc
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
		case oplt:
			env.push(env.pop().(int) < env.pop().(int))
		case opincr:
			env.push(env.pop().(int) + 1)
		case opload:
			xs := code.v.([2]int)
			env.push(env.value[env.scopeOffset(xs[0])-xs[1]])
		case opstore:
			xs := code.v.([2]int)
			i := env.scopeOffset(xs[0]) - xs[1]
			if i >= len(env.value) {
				vs := make([]interface{}, (i+1)*2)
				copy(vs, env.value)
				env.value = vs
			}
			env.value[i] = env.pop()
		case opfork:
			env.pushfork(code.op, code.v.(int))
		case opbacktrack:
			pc++
			break loop
		case opjump:
			pc = code.v.(int)
		case opjumppop:
			pc = env.pop().(int)
		case opjumpifnot:
			if !valueToBool(env.pop()) {
				pc = code.v.(int)
			}
		case opret:
			if env.scopes.top().(scope).id == 0 {
				env.pc = len(env.codes)
				return env.pop(), true
			}
			env.scopes.pop()
			pc = env.scopes.top().(scope).pc
		case opcall:
			xs := code.v.([2]interface{})
			switch v := xs[0].(type) {
			case int:
				env.pushfork(code.op, pc+1)
				pc = v
			case string:
				argcnt := xs[1].(int)
				x, args := env.pop(), make([]interface{}, argcnt)
				for i := argcnt - 1; i >= 0; i-- {
					args[i] = env.pop()
				}
				env.push(internalFuncs[v].callback(x, args))
			default:
				panic(v)
			}
		case opscope:
			xs := code.v.([2]int)
			offset := -1
			if !env.scopes.empty() {
				offset = env.scopes.top().(scope).offset
			}
			env.scopes.push(scope{xs[0], offset + xs[1], 0})
		case oparray:
			x, y := env.pop(), env.pop()
			env.push(append(y.([]interface{}), x))
		case opindex:
			x, y := env.pop(), env.pop()
			env.push(y.([]interface{})[x.(int)])
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
