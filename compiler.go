package gojq

import "errors"

type compiler struct {
	codes  []*code
	offset int
	varcnt int
	funcs  []funcinfo
}

type bytecode struct {
	codes  []*code
	varcnt int
}

type funcinfo struct {
	name string
	pc   int
}

func compile(q *Query) (*bytecode, error) {
	return (&compiler{}).compile(q)
}

func (c *compiler) compile(q *Query) (*bytecode, error) {
	if err := c.compileQuery(q); err != nil {
		return nil, err
	}
	return &bytecode{c.codes, c.varcnt}, nil
}

func (c *compiler) newVariable() int {
	i := c.varcnt
	c.varcnt++
	return i
}

func (c *compiler) compileQuery(q *Query) error {
	for _, fd := range q.FuncDefs {
		if err := c.compileFuncDef(fd); err != nil {
			return err
		}
	}
	if err := c.compilePipe(q.Pipe); err != nil {
		return err
	}
	c.append(&code{op: opret})
	c.optimizeJumps()
	c.optimizeNop()
	return nil
}

func (c *compiler) compileFuncDef(e *FuncDef) error {
	return c.lazyCode(
		func() (*code, error) {
			return &code{op: opjump, v: c.pc() - 1}, nil
		},
		func() error {
			pc := c.pc()
			c.funcs = append(c.funcs, funcinfo{e.Name, pc - 1})
			cc := &compiler{offset: pc, varcnt: c.varcnt, funcs: c.funcs}
			bs, err := cc.compile(e.Body)
			if err != nil {
				return err
			}
			c.codes = append(c.codes, bs.codes...)
			c.varcnt = bs.varcnt
			return nil
		},
	)
}

func (c *compiler) compilePipe(e *Pipe) error {
	for _, e := range e.Commas {
		if err := c.compileComma(e); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileComma(e *Comma) error {
	if len(e.Alts) == 1 {
		return c.compileAlt(e.Alts[0])
	}
	if err := c.lazyCode(
		func() (*code, error) { return &code{op: opfork, v: c.pc() + 1}, nil },
		func() error { return c.compileComma(&Comma{e.Alts[:len(e.Alts)-1]}) },
	); err != nil {
		return err
	}
	return c.lazyCode(
		func() (*code, error) { return &code{op: opjump, v: c.pc() - 1}, nil },
		func() error { return c.compileAlt(e.Alts[len(e.Alts)-1]) },
	)
}

func (c *compiler) compileAlt(e *Alt) error {
	if len(e.Right) == 0 {
		return c.compileExpr(e.Left)
	}
	c.append(&code{op: oppush, v: false})
	found := c.newVariable()
	c.append(&code{op: opstore, v: found})
	if err := c.lazyCode(
		func() (*code, error) {
			return &code{op: opfork, v: c.pc() + 7}, nil // opload found
		},
		func() error { return c.compileExpr(e.Left) },
	); err != nil {
		return err
	}
	c.append(&code{op: opdup})
	c.append(&code{op: opjumpifnot, v: c.pc() + 3}) // oppop
	c.append(&code{op: oppush, v: true})            // found some value
	c.append(&code{op: opstore, v: found})
	return c.lazyCode(
		func() (*code, error) {
			return &code{op: opjump, v: c.pc() - 1}, nil // ret
		},
		func() error {
			c.append(&code{op: oppop})
			c.append(&code{op: opbacktrack})
			c.append(&code{op: opload, v: found})
			c.append(&code{op: opjumpifnot, v: c.pc() + 2})
			c.append(&code{op: opbacktrack}) // if found, backtrack
			c.append(&code{op: oppop})
			return c.compileAlt(&Alt{e.Right[0].Right, e.Right[1:]})
		},
	)
}

func (c *compiler) compileExpr(e *Expr) error {
	if e.Bind != nil || e.Label != nil {
		return errors.New("compileExpr")
	}
	if e.Logic != nil {
		return c.compileLogic(e.Logic)
	}
	if e.If != nil {
		return c.compileIf(e.If)
	}
	return errors.New("compileExpr")
}

func (c *compiler) compileLogic(e *Logic) error {
	if len(e.Right) > 0 {
		return errors.New("compileLogic")
	}
	return c.compileAndExpr(e.Left)
}

func (c *compiler) compileIf(e *If) error {
	c.append(&code{op: opdup})
	idx := c.newVariable()
	c.append(&code{op: opstore, v: idx}) // store the current value for then or else clause
	if err := c.compilePipe(e.Cond); err != nil {
		return err
	}
	if err := c.lazyCode(
		func() (*code, error) {
			return &code{op: opjumpifnot, v: c.pc()}, nil // if falsy, skip then clause
		},
		func() error {
			c.append(&code{op: opload, v: idx})
			return c.compilePipe(e.Then)
		},
	); err != nil {
		return err
	}
	return c.lazyCode(
		func() (*code, error) {
			return &code{op: opjump, v: c.pc() - 1}, nil // jump to ret after then clause
		},
		func() error {
			c.append(&code{op: opload, v: idx})
			if len(e.Elif) > 0 {
				return c.compileIf(&If{e.Elif[0].Cond, e.Elif[0].Then, e.Elif[1:], e.Else})
			}
			if e.Else != nil {
				return c.compilePipe(e.Else)
			}
			return nil
		},
	)
}

func (c *compiler) compileAndExpr(e *AndExpr) error {
	if len(e.Right) > 0 {
		return errors.New("compileAndExpr")
	}
	return c.compileCompare(e.Left)
}

func (c *compiler) compileCompare(e *Compare) error {
	if e.Right != nil {
		return errors.New("compileCompare")
	}
	return c.compileArith(e.Left)
}

func (c *compiler) compileArith(e *Arith) error {
	if e.Right != nil {
		return errors.New("compileArith")
	}
	return c.compileFactor(e.Left)
}

func (c *compiler) compileFactor(e *Factor) error {
	if len(e.Right) > 0 {
		return errors.New("compileFactor")
	}
	return c.compileTerm(e.Left)
}

func (c *compiler) compileTerm(e *Term) (err error) {
	defer func() {
		for _, s := range e.SuffixList {
			if err != nil {
				break
			}
			err = c.compileSuffix(s)
		}
	}()
	if e.Identity {
		return nil
	}
	if e.Func != nil {
		return c.compileFunc(e.Func)
	}
	if e.Array != nil {
		return c.compileArray(e.Array)
	}
	if e.Number != nil {
		c.append(&code{op: opconst, v: *e.Number})
		return nil
	}
	if e.Null {
		c.append(&code{op: opconst, v: nil})
		return nil
	}
	if e.True {
		c.append(&code{op: opconst, v: true})
		return nil
	}
	if e.False {
		c.append(&code{op: opconst, v: false})
		return nil
	}
	if e.Pipe != nil {
		return c.compilePipe(e.Pipe)
	}
	return errors.New("compileTerm")
}

func (c *compiler) compileFunc(e *Func) error {
	for i := len(c.funcs) - 1; i >= 0; i-- {
		f := c.funcs[i]
		if f.name == e.Name && f.argcnt == len(e.Args) {
			c.append(&code{op: opcall, v: f.pc})
			return nil
		}
	}
	if fn, ok := internalFuncs[e.Name]; ok && len(e.Args) == 0 && fn.argcount == argcount0 {
		if e.Name == "empty" {
			c.append(&code{op: oppop})
			c.append(&code{op: opbacktrack})
			return nil
		}
		c.append(&code{op: opcall, v: e.Name})
		return nil
	}
	return errors.New("compileFunc")
}

func (c *compiler) compileArray(e *Array) error {
	if e.Pipe == nil {
		c.append(&code{op: opconst, v: []interface{}{}})
		return nil
	}
	c.append(&code{op: oppush, v: []interface{}{}})
	c.append(&code{op: opswap})
	return c.lazyCode(
		func() (*code, error) {
			return &code{op: opfork, v: c.pc() - 1}, nil
		},
		func() error {
			if err := c.compilePipe(e.Pipe); err != nil {
				return err
			}
			c.append(&code{op: oparray})
			c.append(&code{op: opbacktrack})
			c.append(&code{op: oppop})
			return nil
		},
	)
}

func (c *compiler) compileSuffix(e *Suffix) error {
	if e.Iter {
		return c.compileIter()
	}
	return errors.New("compileSuffix")
}

func (c *compiler) compileIter() error {
	length, idx := c.newVariable(), c.newVariable()
	c.append(&code{op: opcall, v: "_toarray"})
	c.append(&code{op: opdup})
	c.append(&code{op: opcall, v: "length"})
	c.append(&code{op: opstore, v: length})
	c.append(&code{op: oppush, v: 0})
	c.append(&code{op: opstore, v: idx})
	c.append(&code{op: opload, v: length})
	c.append(&code{op: opload, v: idx})
	c.append(&code{op: oplt})
	c.append(&code{op: opjumpifnot, v: c.pc() + 7}) // oppop
	c.append(&code{op: opfork, v: c.pc() - 4})      // opload length
	c.append(&code{op: opload, v: idx})
	c.append(&code{op: opindex})
	c.append(&code{op: opload, v: idx})
	c.append(&code{op: opincr})
	c.append(&code{op: opstore, v: idx})
	c.append(&code{op: opjump, v: c.pc() + 2})
	c.append(&code{op: oppop})
	c.append(&code{op: opbacktrack})
	return nil
}

func (c *compiler) append(code *code) {
	c.codes = append(c.codes, code)
}

func (c *compiler) pc() int {
	return c.offset + len(c.codes)
}

func (c *compiler) lazyCode(f func() (*code, error), g func() error) error {
	i := len(c.codes)
	c.codes = append(c.codes, &code{})
	err := g()
	if err != nil {
		return err
	}
	c.codes[i], err = f()
	return err
}

func (c *compiler) optimizeJumps() {
	for i := len(c.codes) - 1; i >= 0; i-- {
		code := c.codes[i]
		if code.op != opjump {
			continue
		}
		for {
			d := c.codes[code.v.(int)+1-c.offset]
			if d.op != opjump {
				break
			}
			code.v = d.v
		}
	}
}

func (c *compiler) optimizeNop() {
	for i, code := range c.codes {
		if code.op == opjump && code.v.(int) == i {
			c.codes[i].op = opnop
		}
	}
}
