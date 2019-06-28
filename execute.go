package gojq

func (env *env) execute(bc *bytecode, v interface{}) Iter {
	env.codes = bc.codes
	env.value = make([]interface{}, bc.varcnt)
	env.push(v)
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	pc, depth := env.pc, env.depth
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
			env.push(env.value[code.v.(int)])
		case opstore:
			env.value[code.v.(int)] = env.pop()
		case opfork:
			env.pushfork(code.op, code.v.(int), env.stack[len(env.stack)-1])
		case opbacktrack:
			pc++
			break loop
		case opjump:
			pc = code.v.(int)
		case opjumpifnot:
			if !valueToBool(env.pop()) {
				pc = code.v.(int)
			}
		case opret:
			if depth > 0 {
				break loop
			}
			env.pc = pc + 1
			return env.pop(), true
		case opcall:
			switch v := code.v.(type) {
			case int:
				env.pushfork(code.op, pc+1, env.stack[len(env.stack)-1])
				pc = v
				depth++
				env.depth = depth
			case string:
				env.push(internalFuncs[v].callback(nil, nil)(env.pop()))
			default:
				panic(v)
			}
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
		if depth != f.depth {
			depth = f.depth
			env.depth = depth
		}
		if f.op != opcall {
			env.push(f.v)
		}
		goto loop
	}
	return nil, false
}

func (env *env) push(v interface{}) {
	env.stack = append(env.stack, v)
}

func (env *env) pop() interface{} {
	v := env.stack[len(env.stack)-1]
	env.stack = env.stack[:len(env.stack)-1]
	return v
}

func (env *env) pushfork(op opcode, pc int, v interface{}) {
	env.forks = append(env.forks, &fork{op, pc, v, env.depth})
	if debug {
		env.debugForks(pc, ">>>")
	}
}

func (env *env) popfork() *fork {
	v := env.forks[len(env.forks)-1]
	if debug {
		env.debugForks(v.pc, "<<<")
	}
	env.forks = env.forks[:len(env.forks)-1]
	return v
}
