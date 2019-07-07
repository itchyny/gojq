package gojq

import (
	"errors"
	"fmt"
)

type compiler struct {
	codes     []*code
	codeinfos []*codeinfo
	offset    int
	scopes    []*scopeinfo
	scopecnt  int
	funcs     []funcinfo
}

type bytecode struct {
	codes     []*code
	codeinfos []*codeinfo
}

type codeinfo struct {
	name string
	pc   int
}

type funcinfo struct {
	name   string
	argcnt int
	pc     int
}

type scopeinfo struct {
	id        int
	offset    int
	variables []varinfo
}

type varinfo struct {
	name  string
	index [2]int
}

func compile(q *Query) (*bytecode, error) {
	c := &compiler{}
	scope := c.newScope()
	c.scopes = []*scopeinfo{scope}
	defer c.lazy(func() *code {
		return &code{op: opscope, v: [2]int{scope.id, len(scope.variables)}}
	})()
	return c.compile(q)
}

func (c *compiler) compile(q *Query) (*bytecode, error) {
	if err := c.compileQuery(q); err != nil {
		return nil, err
	}
	return &bytecode{c.codes, c.codeinfos}, nil
}

func (c *compiler) newVariable() [2]int {
	return c.pushVariable("")
}

func (c *compiler) pushVariable(name string) [2]int {
	s := c.scopes[len(c.scopes)-1]
	i := len(s.variables)
	v := [2]int{s.id, i}
	s.variables = append(s.variables, varinfo{name, v})
	return v
}

func (c *compiler) newScope() *scopeinfo {
	i := c.scopecnt // do not use len(c.scopes) because it pops
	c.scopecnt++
	return &scopeinfo{i, 0, nil}
}

func (c *compiler) compileQuery(q *Query) error {
	for _, fd := range q.FuncDefs {
		if err := c.compileFuncDef(fd, false); err != nil {
			return err
		}
	}
	if q.Pipe != nil {
		if err := c.compilePipe(q.Pipe); err != nil {
			return err
		}
	}
	c.append(&code{op: opret})
	c.optimizeNop()
	c.optimizeJumps()
	return nil
}

func (c *compiler) compileFuncDef(e *FuncDef, builtin bool) error {
	if builtin {
		for i := len(c.funcs) - 1; i >= 0; i-- {
			f := c.funcs[i]
			if f.name == e.Name && f.argcnt == len(e.Args) {
				return nil
			}
		}
	}
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc() - 1}
	})()
	c.appendCodeInfo(e.Name)
	defer c.appendCodeInfo(e.Name)
	pc := c.pc()
	c.funcs = append(c.funcs, funcinfo{e.Name, len(e.Args), pc - 1})
	cc := &compiler{offset: pc, scopecnt: c.scopecnt, funcs: c.funcs}
	scope := cc.newScope()
	cc.scopes = append(c.scopes, scope)
	setscope := cc.lazy(func() *code {
		return &code{op: opscope, v: [2]int{scope.id, len(scope.variables)}}
	})
	if len(e.Args) > 0 {
		v := cc.newVariable()
		cc.append(&code{op: opstore, v: v})
		for _, name := range e.Args {
			if name[0] == '$' {
				cc.append(&code{op: opload, v: v})
				cc.append(&code{op: opswap})
				cc.append(&code{op: opjumppop})
			}
			cc.append(&code{op: opstore, v: cc.pushVariable(name)})
		}
		cc.append(&code{op: opload, v: v})
	}
	bs, err := cc.compile(e.Body)
	if err != nil {
		return err
	}
	setscope()
	c.codes = append(c.codes, bs.codes...)
	c.codeinfos = append(c.codeinfos, bs.codeinfos...)
	c.scopecnt = cc.scopecnt
	return nil
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
	setfork := c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() + 1}
	})
	if err := c.compileComma(&Comma{e.Alts[:len(e.Alts)-1]}); err != nil {
		return err
	}
	setfork()
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc() - 1}
	})()
	return c.compileAlt(e.Alts[len(e.Alts)-1])
}

func (c *compiler) compileAlt(e *Alt) error {
	if len(e.Right) == 0 {
		return c.compileExpr(e.Left)
	}
	c.append(&code{op: oppush, v: false})
	found := c.newVariable()
	c.append(&code{op: opstore, v: found})
	setfork := c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() + 7} // opload found
	})
	if err := c.compileExpr(e.Left); err != nil {
		return err
	}
	setfork()
	c.append(&code{op: opdup})
	c.append(&code{op: opjumpifnot, v: c.pc() + 3}) // oppop
	c.append(&code{op: oppush, v: true})            // found some value
	c.append(&code{op: opstore, v: found})
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc() - 1} // ret
	})()
	c.append(&code{op: oppop})
	c.append(&code{op: opbacktrack})
	c.append(&code{op: opload, v: found})
	c.append(&code{op: opjumpifnot, v: c.pc() + 2})
	c.append(&code{op: opbacktrack}) // if found, backtrack
	c.append(&code{op: oppop})
	return c.compileAlt(&Alt{e.Right[0].Right, e.Right[1:]})
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
	setjumpifnot := c.lazy(func() *code {
		return &code{op: opjumpifnot, v: c.pc()} // if falsy, skip then clause
	})
	c.append(&code{op: opload, v: idx})
	if err := c.compilePipe(e.Then); err != nil {
		return err
	}
	setjumpifnot()
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc() - 1} // jump to ret after else clause
	})()
	c.append(&code{op: opload, v: idx})
	if len(e.Elif) > 0 {
		return c.compileIf(&If{e.Elif[0].Cond, e.Elif[0].Then, e.Elif[1:], e.Else})
	}
	if e.Else != nil {
		return c.compilePipe(e.Else)
	}
	return nil
}

func (c *compiler) compileAndExpr(e *AndExpr) error {
	if len(e.Right) > 0 {
		return errors.New("compileAndExpr")
	}
	return c.compileCompare(e.Left)
}

func (c *compiler) compileCompare(e *Compare) error {
	if e.Right == nil {
		return c.compileArith(e.Left)
	}
	return c.compileCall(
		e.Right.Op.getFunc(),
		[]*Pipe{e.Left.toPipe(), e.Right.Right.toPipe()},
	)
}

func (c *compiler) compileArith(e *Arith) error {
	if len(e.Right) == 0 {
		return c.compileFactor(e.Left)
	}
	r := e.Right[len(e.Right)-1]
	return c.compileCall(
		r.Op.getFunc(),
		[]*Pipe{
			(&Arith{e.Left, e.Right[:len(e.Right)-1]}).toPipe(),
			r.Right.toPipe(),
		},
	)
}

func (c *compiler) compileFactor(e *Factor) error {
	if len(e.Right) == 0 {
		return c.compileTerm(e.Left)
	}
	r := e.Right[len(e.Right)-1]
	return c.compileCall(
		r.Op.getFunc(),
		[]*Pipe{
			(&Factor{e.Left, e.Right[:len(e.Right)-1]}).toPipe(),
			r.Right.toPipe(),
		},
	)
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
	for i := len(c.scopes) - 1; i >= 0; i-- {
		s := c.scopes[i]
		for j := len(s.variables) - 1; j >= 0; j-- {
			v := s.variables[j]
			if v.name == e.Name && len(e.Args) == 0 {
				if e.Name[0] == '$' {
					c.append(&code{op: oppop})
					c.append(&code{op: opload, v: v.index})
				} else {
					c.append(&code{op: opload, v: v.index})
					c.append(&code{op: opjumppop})
				}
				return nil
			}
		}
	}
	for i := len(c.funcs) - 1; i >= 0; i-- {
		f := c.funcs[i]
		if f.name == e.Name && f.argcnt == len(e.Args) {
			if err := c.compileCall(f.pc, e.Args); err != nil {
				return err
			}
			return nil
		}
	}
	if q, ok := builtinFuncs[e.Name]; ok {
		for _, fd := range q.FuncDefs {
			if err := c.compileFuncDef(fd, true); err != nil {
				return err
			}
		}
		for i := len(c.funcs) - 1; i >= 0; i-- {
			f := c.funcs[i]
			if f.name == e.Name && f.argcnt == len(e.Args) {
				if err := c.compileCall(f.pc, e.Args); err != nil {
					return err
				}
				return nil
			}
		}
	}
	if fn, ok := internalFuncs[e.Name]; ok && fn.accept(len(e.Args)) {
		if e.Name == "empty" {
			c.append(&code{op: oppop})
			c.append(&code{op: opbacktrack})
			return nil
		}
		if err := c.compileCall(e.Name, e.Args); err != nil {
			return err
		}
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
	arr := c.newVariable()
	c.append(&code{op: opstore, v: arr})
	defer c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() - 2}
	})()
	if err := c.compilePipe(e.Pipe); err != nil {
		return err
	}
	c.append(&code{op: opappend, v: arr})
	c.append(&code{op: opbacktrack})
	c.append(&code{op: oppop})
	c.append(&code{op: opload, v: arr})
	return nil
}

func (c *compiler) compileSuffix(e *Suffix) error {
	if e.Iter {
		return c.compileIter()
	}
	return errors.New("compileSuffix")
}

func (c *compiler) compileIter() error {
	c.append(&code{op: opeach})
	c.append(&code{op: opbacktrack})
	return nil
}

func (c *compiler) compileCall(fn interface{}, args []*Pipe) error {
	if len(args) == 0 {
		c.append(&code{op: opcall, v: [2]interface{}{fn, len(args)}})
		return nil
	}
	idx := c.newVariable()
	c.append(&code{op: opstore, v: idx})
	for i := len(args) - 1; i >= 0; i-- {
		pc := c.pc() // ref: compileFuncDef
		if err := c.compileFuncDef(&FuncDef{Name: fmt.Sprintf("lambda:%d", pc+1), Body: &Query{Pipe: args[i]}}, false); err != nil {
			return err
		}
		if _, ok := fn.(string); ok {
			c.append(&code{op: opload, v: idx})
			c.append(&code{op: oppush, v: pc})
			c.append(&code{op: opjumppop})
		} else {
			c.append(&code{op: oppush, v: pc})
		}
	}
	c.append(&code{op: opload, v: idx})
	c.append(&code{op: opcall, v: [2]interface{}{fn, len(args)}})
	return nil
}

func (c *compiler) append(code *code) {
	c.codes = append(c.codes, code)
}

func (c *compiler) pc() int {
	return c.offset + len(c.codes)
}

func (c *compiler) lazy(f func() *code) func() {
	i := len(c.codes)
	c.codes = append(c.codes, &code{op: opnop})
	return func() { c.codes[i] = f() }
}

func (c *compiler) optimizeNop() {
	for i, code := range c.codes {
		if code.op == opjump && code.v.(int) == i {
			c.codes[i].op = opnop
		}
	}
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
