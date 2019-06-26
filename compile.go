package gojq

import "errors"

func (env *env) compileQuery(q *Query) error {
	if len(q.FuncDefs) > 0 {
		return errors.New("funcdef")
	}
	if err := env.compilePipe(q.Pipe); err != nil {
		return err
	}
	env.append(&code{op: opret})
	return nil
}

func (env *env) compilePipe(e *Pipe) error {
	for _, e := range e.Commas {
		if err := env.compileComma(e); err != nil {
			return err
		}
	}
	return nil
}

func (env *env) compileComma(e *Comma) error {
	return env.compileAlts(e.Alts)
}

func (env *env) compileAlts(xs []*Alt) error {
	if len(xs) == 1 {
		return env.compileAlt(xs[0])
	}
	if err := env.compileLazy(
		func() (*code, error) { return &code{op: opfork, v: len(env.codes) + 1}, nil },
		func() error { return env.compileAlts(xs[:len(xs)-1]) },
	); err != nil {
		return err
	}
	return env.compileLazy(
		func() (*code, error) { return &code{op: opjump, v: len(env.codes) - 1}, nil },
		func() error { return env.compileAlt(xs[len(xs)-1]) },
	)
}

func (env *env) compileAlt(e *Alt) error {
	if len(e.Right) > 0 {
		return errors.New("compileAlt")
	}
	return env.compileExpr(e.Left)
}

func (env *env) compileExpr(e *Expr) error {
	if e.Bind != nil || e.Label != nil {
		return errors.New("compileExpr")
	}
	if e.Logic != nil {
		return env.compileLogic(e.Logic)
	}
	if e.If != nil {
		return env.compileIf(e.If)
	}
	return errors.New("compileExpr")
}

func (env *env) compileLogic(e *Logic) error {
	if len(e.Right) > 0 {
		return errors.New("compileLogic")
	}
	return env.compileAndExpr(e.Left)
}

func (env *env) compileIf(e *If) error {
	env.append(&code{op: opdup})
	idx := env.newVariable()
	env.append(&code{op: opstore, v: idx}) // store the current value for then or else clause
	if err := env.compilePipe(e.Cond); err != nil {
		return err
	}
	if err := env.compileLazy(
		func() (*code, error) {
			return &code{op: opjumpifnot, v: len(env.codes)}, nil // if falsy, skip then clause
		},
		func() error {
			env.append(&code{op: opload, v: idx})
			return env.compilePipe(e.Then)
		},
	); err != nil {
		return err
	}
	return env.compileLazy(
		func() (*code, error) {
			return &code{op: opjump, v: len(env.codes) - 1}, nil // jump to ret after then clause
		},
		func() error {
			env.append(&code{op: opload, v: idx})
			if len(e.Elif) > 0 {
				return env.compileIf(&If{e.Elif[0].Cond, e.Elif[0].Then, e.Elif[1:], e.Else})
			}
			if e.Else != nil {
				return env.compilePipe(e.Else)
			}
			return nil
		},
	)
}

func (env *env) compileAndExpr(e *AndExpr) error {
	if len(e.Right) > 0 {
		return errors.New("compileAndExpr")
	}
	return env.compileCompare(e.Left)
}

func (env *env) compileCompare(e *Compare) error {
	if e.Right != nil {
		return errors.New("compileCompare")
	}
	return env.compileArith(e.Left)
}

func (env *env) compileArith(e *Arith) error {
	if e.Right != nil {
		return errors.New("compileArith")
	}
	return env.compileFactor(e.Left)
}

func (env *env) compileFactor(e *Factor) error {
	if len(e.Right) > 0 {
		return errors.New("compileFactor")
	}
	return env.compileTerm(e.Left)
}

func (env *env) compileTerm(e *Term) error {
	if len(e.SuffixList) > 0 {
		return errors.New("compileTerm")
	}
	if e.Identity {
		return nil
	}
	if e.Array != nil {
		return env.compileArray(e.Array)
	}
	if e.Number != nil {
		env.append(&code{op: opconst, v: *e.Number})
		return nil
	}
	if e.Null {
		env.append(&code{op: opconst, v: nil})
		return nil
	}
	if e.True {
		env.append(&code{op: opconst, v: true})
		return nil
	}
	if e.False {
		env.append(&code{op: opconst, v: false})
		return nil
	}
	if e.Pipe != nil {
		return env.compilePipe(e.Pipe)
	}
	return errors.New("compileTerm")
}

func (env *env) compileArray(e *Array) error {
	if e.Pipe == nil {
		env.append(&code{op: opconst, v: []interface{}{}})
		return nil
	}
	env.append(&code{op: oppush, v: []interface{}{}})
	env.append(&code{op: opswap})
	return env.compileLazy(
		func() (*code, error) {
			return &code{op: opfork, v: len(env.codes) - 1}, nil
		},
		func() error {
			if err := env.compilePipe(e.Pipe); err != nil {
				return err
			}
			env.append(&code{op: oparray})
			env.append(&code{op: opbacktrack})
			env.append(&code{op: oppop})
			return nil
		},
	)
}

func (env *env) append(c *code) {
	env.codes = append(env.codes, c)
}

func (env *env) compileLazy(f func() (*code, error), g func() error) error {
	i := len(env.codes)
	env.codes = append(env.codes, &code{})
	err := g()
	if err != nil {
		return err
	}
	env.codes[i], err = f()
	return err
}
