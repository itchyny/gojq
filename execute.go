package gojq

func (env *env) execute(bc *bytecode, v interface{}) Iter {
	env.codes = bc.codes
	env.codeinfos = bc.codeinfos
	env.push(v)
	env.debugCodes()
	return env
}

func (env *env) Next() (interface{}, bool) {
	var err error
	pc, callpc, backtrack := env.pc, len(env.codes)-1, env.backtrack
	defer func() { env.pc, env.backtrack = pc, true }()
loop:
	for ; 0 <= pc && pc < len(env.codes); pc++ {
		env.debugState(pc, backtrack)
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
		case opobject:
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
			env.value[i] = append(env.value[i].([]interface{}), env.pop())
		case opfork:
			if backtrack || err != nil {
				pc, backtrack = code.v.(int), false
				goto loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opforkopt:
			if backtrack || err != nil {
				pc, backtrack, err = code.v.(int), false, nil
				goto loop
			} else {
				env.pushfork(code.op, pc)
			}
		case opbacktrack:
			break loop
		case opjump:
			pc = code.v.(int)
			goto loop
		case opjumppop:
			pc, callpc = env.pop().(int), pc
			goto loop
		case opjumpifnot:
			if !valueToBool(env.pop()) {
				pc = code.v.(int)
				goto loop
			}
		case opret:
			if backtrack || err != nil {
				break loop
			}
			pc = env.scopes.pop().(scope).pc
			if env.scopes.empty() {
				if env.stack.empty() {
					return nil, false
				}
				return env.pop(), true
			}
		case opcall:
			switch v := code.v.(type) {
			case int:
				pc, callpc = v, pc
				goto loop
			case [3]interface{}:
				argcnt := v[1].(int)
				x, args := env.pop(), make([]interface{}, argcnt)
				for i := 0; i < argcnt; i++ {
					args[i] = env.pop()
				}
				w := v[0].(func(interface{}, []interface{}) interface{})(x, args)
				if e, ok := w.(error); ok {
					err = e
					break loop
				}
				env.push(w)
			default:
				panic(v)
			}
		case opscope:
			xs := code.v.([2]int)
			env.scopes.push(scope{xs[0], env.offset, callpc})
			env.offset += xs[1]
		case opeach:
			if err != nil {
				break loop
			}
			backtrack = false
			switch v := env.pop().(type) {
			case []interface{}:
				if len(v) == 0 {
					break loop
				}
				if len(v) > 1 {
					env.push(v[1:])
					env.pushfork(code.op, pc)
					env.pop()
				}
				env.push(v[0])
			case map[string]interface{}:
				if len(v) == 0 {
					break loop
				}
				a := make([]interface{}, len(v))
				var i int
				for _, v := range v {
					a[i] = v
					i++
				}
				if len(v) > 1 {
					env.push(a[1:])
					env.pushfork(code.op, pc)
					env.pop()
				}
				env.push(a[0])
			default:
				err = &iteratorError{v}
				break loop
			}
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
	return env.scopeOffset(v[0]) + v[1]
}
