package gojq

func (env *env) execute(v interface{}) Iter {
	env.push(v)
	return env
}

func (env *env) Next() (interface{}, bool) {
	pc := env.pc
loop:
	for ; 0 <= pc && pc < len(env.codes); pc++ {
		c := env.codes[pc]
		switch c.op {
		case opload:
			env.push(c.v)
		case opconst:
			env.pop()
			env.push(c.v)
		case opret:
			pc++
			break loop
		default:
			panic(c.op)
		}
	}
	env.pc = pc
	if len(env.stack) == 0 {
		return nil, false
	}
	return env.pop(), true
}

func (env *env) push(v interface{}) {
	env.stack = append(env.stack, v)
}

func (env *env) pop() interface{} {
	v := env.stack[len(env.stack)-1]
	env.stack = env.stack[:len(env.stack)-1]
	return v
}
