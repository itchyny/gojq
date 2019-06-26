package gojq

func (env *env) execute(v interface{}) Iter {
	env.push(v)
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	pc := env.pc
loop:
	for ; 0 <= pc && pc < len(env.codes); pc++ {
		env.debugState(pc)
		c := env.codes[pc]
		switch c.op {
		case oppush:
			env.push(c.v)
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
			env.push(c.v)
		case opload:
			env.push(env.value[c.v.(int)])
		case opstore:
			env.value[c.v.(int)] = env.pop()
		case opfork:
			env.pushfork(c.v.(int), env.stack[len(env.stack)-1])
		case opbacktrack:
			pc++
			break loop
		case opjump:
			pc = c.v.(int)
		case opjumpifnot:
			if !valueToBool(env.pop()) {
				pc = c.v.(int)
			}
		case opret:
			env.pc = pc + 1
			return env.pop(), true
		case opcall:
			env.push(internalFuncs[c.v.(string)].callback(nil, nil)(env.pop()))
		case oparray:
			x, y := env.pop(), env.pop()
			env.push(append(y.([]interface{}), x))
		default:
			panic(c.op)
		}
	}
	env.pc = pc
	if len(env.forks) > 0 {
		f := env.popfork()
		pc = f.pc
		env.push(f.v)
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

func (env *env) pushfork(pc int, v interface{}) {
	env.forks = append(env.forks, &fork{pc, v})
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
