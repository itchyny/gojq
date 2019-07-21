package gojq

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type compiler struct {
	codes      []*code
	codeinfos  []codeinfo
	codeoffset int
	scopes     []*scopeinfo
	scopecnt   int
	funcs      []*funcinfo
}

type bytecode struct {
	codes     []*code
	codeinfos []codeinfo
}

type codeinfo struct {
	name string
	pc   int
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

type funcinfo struct {
	name      string
	pc        int
	args      []string
	argsorder []int
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
	c.optimizeJumps()
	return nil
}

func (c *compiler) compileFuncDef(e *FuncDef, builtin bool) error {
	if builtin {
		for i := len(c.funcs) - 1; i >= 0; i-- {
			f := c.funcs[i]
			if f.name == e.Name && len(f.args) == len(e.Args) {
				return nil
			}
		}
	}
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()}
	})()
	c.appendCodeInfo(e.Name)
	defer c.appendCodeInfo(e.Name)
	pc, argsorder := c.pc(), getArgsOrder(e.Args)
	c.funcs = append(c.funcs, &funcinfo{e.Name, pc, e.Args, argsorder})
	cc := &compiler{codeoffset: pc, scopecnt: c.scopecnt, funcs: c.funcs}
	scope := cc.newScope()
	cc.scopes = append(c.scopes, scope)
	setscope := cc.lazy(func() *code {
		return &code{op: opscope, v: [2]int{scope.id, len(scope.variables)}}
	})
	if len(e.Args) > 0 {
		v := cc.newVariable()
		cc.append(&code{op: opstore, v: v})
		skip := make([]bool, len(e.Args))
		for i, name := range e.Args {
			for j := 0; j < i; j++ {
				if name == e.Args[j] {
					skip[j] = true
					break
				}
			}
		}
		for _, i := range argsorder {
			if skip[i] {
				cc.append(&code{op: oppop})
			} else {
				cc.append(&code{op: opstore, v: cc.pushVariable(e.Args[i])})
			}
		}
		cc.append(&code{op: opload, v: v})
	}
	bs, err := cc.compile(e.Body)
	if err != nil {
		return err
	}
	setscope()
	cc.optimizeTailRec()
	c.codes = append(c.codes, bs.codes...)
	c.codeinfos = append(c.codeinfos, bs.codeinfos...)
	c.scopecnt = cc.scopecnt
	return nil
}

func getArgsOrder(args []string) []int {
	xs := make([]int, len(args))
	if len(xs) > 0 {
		for i := range xs {
			xs[i] = i
		}
		sort.Slice(xs, func(i, j int) bool {
			xi, xj := xs[i], xs[j]
			if args[xi][0] == '$' {
				return args[xj][0] == '$' && xi > xj // reverse the order of variables
			}
			return args[xj][0] == '$' || xi < xj
		})
	}
	return xs
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
		return &code{op: opjump, v: c.pc()}
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
		return &code{op: opfork, v: c.pc()} // opload found
	})
	if err := c.compileExpr(e.Left); err != nil {
		return err
	}
	c.append(&code{op: opdup})
	c.append(&code{op: opjumpifnot, v: c.pc() + 4}) // oppop
	c.append(&code{op: oppush, v: true})            // found some value
	c.append(&code{op: opstore, v: found})
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()} // ret
	})()
	c.append(&code{op: oppop})
	c.append(&code{op: opbacktrack})
	setfork()
	c.append(&code{op: opload, v: found})
	c.append(&code{op: opjumpifnot, v: c.pc() + 3})
	c.append(&code{op: opbacktrack}) // if found, backtrack
	c.append(&code{op: oppop})
	return c.compileAlt(&Alt{e.Right[0].Right, e.Right[1:]})
}

func (c *compiler) compileExpr(e *Expr) (err error) {
	if e.Update != nil {
		t := *e // clone without changing e
		(&t).Update = nil
		switch e.UpdateOp {
		case OpAssign, OpModify:
			return c.compileFunc(
				&Func{
					Name: e.UpdateOp.getFunc(),
					Args: []*Pipe{
						t.toPipe(),
						e.Update.toPipe(),
					},
				},
			)
		default:
			return c.compileFunc(
				&Func{
					Name: "_modify",
					Args: []*Pipe{
						t.toPipe(),
						(&Term{Func: &Func{
							Name: e.UpdateOp.getFunc(),
							Args: []*Pipe{
								(&Term{Identity: true}).toPipe(),
								e.Update.toPipe(),
							},
						}}).toPipe(),
					},
				},
			)
		}
	}
	if e.Bind != nil {
		c.append(&code{op: opdup})
	}
	defer func() {
		if err != nil {
			return
		}
		if b := e.Bind; b != nil {
			if err = c.compilePattern(b.Pattern); err != nil {
				return
			}
			err = c.compilePipe(b.Body)
			return
		}
	}()
	if e.Logic != nil {
		return c.compileLogic(e.Logic)
	} else if e.If != nil {
		return c.compileIf(e.If)
	} else if e.Try != nil {
		return c.compileTry(e.Try)
	} else if e.Reduce != nil {
		return c.compileReduce(e.Reduce)
	} else if e.Foreach != nil {
		return c.compileForeach(e.Foreach)
	} else if e.Label != nil {
		return c.compileLabel(e.Label)
	} else {
		return fmt.Errorf("invalid expr: %s", e)
	}
}

func (c *compiler) compilePattern(p *Pattern) error {
	c.appendCodeInfo(p)
	if p.Name != "" {
		if p.Name[0] != '$' {
			return &bindVariableNameError{p.Name}
		}
		c.append(&code{op: opstore, v: c.pushVariable(p.Name)})
		return nil
	} else if len(p.Array) > 0 {
		v := c.newVariable()
		c.append(&code{op: opstore, v: v})
		for i, p := range p.Array {
			c.append(&code{op: oppush, v: i})
			c.append(&code{op: opload, v: v})
			c.append(&code{op: opload, v: v})
			// ref: compileCall
			c.append(&code{op: opcall, v: [3]interface{}{internalFuncs["_index"].callback, 2, "_index"}})
			if err := c.compilePattern(p); err != nil {
				return err
			}
		}
		return nil
	} else if len(p.Object) > 0 {
		v := c.newVariable()
		c.append(&code{op: opstore, v: v})
		for _, kv := range p.Object {
			var key, name string
			if kv.KeyOnly != "" {
				if kv.KeyOnly[0] != '$' {
					return &bindVariableNameError{kv.KeyOnly}
				}
				key, name = kv.KeyOnly[1:], kv.KeyOnly
				c.append(&code{op: oppush, v: key})
			} else if kv.Key != "" {
				key = kv.Key
				if key != "" && key[0] == '$' {
					key, name = key[1:], key
				}
				c.append(&code{op: oppush, v: key})
			} else if kv.KeyString != "" {
				key = kv.KeyString[1 : len(kv.KeyString)-1]
				c.append(&code{op: oppush, v: key})
			} else if kv.Pipe != nil {
				c.append(&code{op: opload, v: v})
				if err := c.compilePipe(kv.Pipe); err != nil {
					return err
				}
			}
			c.append(&code{op: opload, v: v})
			c.append(&code{op: opload, v: v})
			// ref: compileCall
			c.append(&code{op: opcall, v: [3]interface{}{internalFuncs["_index"].callback, 2, "_index"}})
			if name != "" {
				if kv.Val != nil {
					c.append(&code{op: opdup})
				}
				if err := c.compilePattern(&Pattern{Name: name}); err != nil {
					return err
				}
			}
			if kv.Val != nil {
				if err := c.compilePattern(kv.Val); err != nil {
					return err
				}
			}
		}
		return nil
	} else {
		return fmt.Errorf("invalid pattern: %s", p)
	}
}

func (c *compiler) compileLogic(e *Logic) error {
	if len(e.Right) == 0 {
		return c.compileAndExpr(e.Left)
	}
	return c.compileIf(
		&If{
			Cond: (&Logic{e.Left, e.Right[:len(e.Right)-1]}).toPipe(),
			Then: (&Term{True: true}).toPipe(),
			Else: (&Expr{If: &If{
				Cond: e.Right[len(e.Right)-1].Right.toPipe(),
				Then: (&Term{True: true}).toPipe(),
				Else: (&Term{False: true}).toPipe(),
			}}).toPipe(),
		},
	)
}

func (c *compiler) compileIf(e *If) error {
	c.appendCodeInfo(e)
	c.append(&code{op: opdup}) // duplicate the value for then or else clause
	if err := c.compilePipe(e.Cond); err != nil {
		return err
	}
	setjumpifnot := c.lazy(func() *code {
		return &code{op: opjumpifnot, v: c.pc() + 1} // if falsy, skip then clause
	})
	if err := c.compilePipe(e.Then); err != nil {
		return err
	}
	setjumpifnot()
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()} // jump to ret after else clause
	})()
	if len(e.Elif) > 0 {
		return c.compileIf(&If{e.Elif[0].Cond, e.Elif[0].Then, e.Elif[1:], e.Else})
	}
	if e.Else != nil {
		return c.compilePipe(e.Else)
	}
	return nil
}

func (c *compiler) compileTry(e *Try) error {
	c.appendCodeInfo(e)
	setforkopt := c.lazy(func() *code {
		return &code{op: opforkopt, v: c.pc()}
	})
	if err := c.compilePipe(e.Body); err != nil {
		return err
	}
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()}
	})()
	setforkopt()
	if e.Catch != nil {
		return c.compilePipe(e.Catch)
	}
	c.append(&code{op: opbacktrack})
	return nil
}

func (c *compiler) compileReduce(e *Reduce) error {
	c.appendCodeInfo(e)
	defer c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() - 2}
	})()
	c.append(&code{op: opdup})
	v := c.newVariable()
	if err := c.compilePipe(e.Start); err != nil {
		return err
	}
	c.append(&code{op: opstore, v: v})
	if err := c.compileTerm(e.Term); err != nil {
		return err
	}
	if err := c.compilePattern(e.Pattern); err != nil {
		return err
	}
	c.append(&code{op: opload, v: v})
	if err := c.compilePipe(e.Update); err != nil {
		return err
	}
	c.append(&code{op: opstore, v: v})
	c.append(&code{op: opbacktrack})
	c.append(&code{op: oppop})
	c.append(&code{op: opload, v: v})
	return nil
}

func (c *compiler) compileForeach(e *Foreach) error {
	c.appendCodeInfo(e)
	c.append(&code{op: opdup})
	v := c.newVariable()
	if err := c.compilePipe(e.Start); err != nil {
		return err
	}
	c.append(&code{op: opstore, v: v})
	if err := c.compileTerm(e.Term); err != nil {
		return err
	}
	if err := c.compilePattern(e.Pattern); err != nil {
		return err
	}
	c.append(&code{op: opload, v: v})
	if err := c.compilePipe(e.Update); err != nil {
		return err
	}
	c.append(&code{op: opdup})
	c.append(&code{op: opstore, v: v})
	if e.Extract != nil {
		return c.compilePipe(e.Extract)
	}
	return nil
}

func (c *compiler) compileLabel(e *Label) error {
	c.appendCodeInfo(e)
	if e.Ident[0] != '$' {
		return &labelNameError{e.Ident}
	}
	defer c.lazy(func() *code {
		return &code{op: opforklabel, v: e.Ident}
	})()
	return c.compilePipe(e.Body)
}

func (c *compiler) compileAndExpr(e *AndExpr) error {
	if len(e.Right) == 0 {
		return c.compileCompare(e.Left)
	}
	return c.compileIf(
		&If{
			Cond: (&AndExpr{e.Left, e.Right[:len(e.Right)-1]}).toPipe(),
			Then: (&Expr{If: &If{
				Cond: e.Right[len(e.Right)-1].Right.toPipe(),
				Then: (&Term{True: true}).toPipe(),
				Else: (&Term{False: true}).toPipe(),
			}}).toPipe(),
			Else: (&Term{False: true}).toPipe(),
		},
	)
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
	if len(e.SuffixList) > 0 {
		s := e.SuffixList[len(e.SuffixList)-1]
		t := *e // clone without changing e
		(&t).SuffixList = t.SuffixList[:len(e.SuffixList)-1]
		return c.compileTermSuffix(&t, s)
	}
	if e.Index != nil {
		return c.compileIndex(&Term{Identity: true}, e.Index)
	} else if e.Identity {
		return nil
	} else if e.Recurse {
		return c.compileFunc(&Func{Name: "recurse"})
	} else if e.Func != nil {
		return c.compileFunc(e.Func)
	} else if e.Object != nil {
		return c.compileObject(e.Object)
	} else if e.Array != nil {
		return c.compileArray(e.Array)
	} else if e.Number != nil {
		c.append(&code{op: opconst, v: *e.Number})
		return nil
	} else if e.Unary != nil {
		return c.compileUnary(e.Unary)
	} else if e.Str != "" {
		return c.compileString(e.Str)
	} else if e.RawStr != "" {
		c.append(&code{op: opconst, v: e.RawStr})
		return nil
	} else if e.Null {
		c.append(&code{op: opconst, v: nil})
		return nil
	} else if e.True {
		c.append(&code{op: opconst, v: true})
		return nil
	} else if e.False {
		c.append(&code{op: opconst, v: false})
		return nil
	} else if e.Break != "" {
		c.append(&code{op: opconst, v: e.Break})
		return c.compileCall("_break", nil)
	} else if e.Pipe != nil {
		return c.compilePipe(e.Pipe)
	} else {
		return fmt.Errorf("invalid term: %s", e)
	}
}

func (c *compiler) compileIndex(e *Term, x *Index) error {
	c.appendCodeInfo(x)
	if x.Name != "" {
		return c.compileCall("_index", []*Pipe{e.toPipe(), (&Term{RawStr: x.Name}).toPipe()})
	}
	if x.Str != "" {
		p, err := c.stringToPipe(x.Str)
		if err != nil {
			return err
		}
		return c.compileCall("_index", []*Pipe{e.toPipe(), p})
	}
	if x.Start != nil {
		if x.IsSlice {
			if x.End != nil {
				return c.compileCall("_slice", []*Pipe{e.toPipe(), x.End, x.Start})
			}
			return c.compileCall("_slice", []*Pipe{e.toPipe(), (&Term{Null: true}).toPipe(), x.Start})
		}
		return c.compileCall("_index", []*Pipe{e.toPipe(), x.Start})
	}
	return c.compileCall("_slice", []*Pipe{e.toPipe(), x.End, (&Term{Null: true}).toPipe()})
}

func (c *compiler) compileFunc(e *Func) error {
	for i := len(c.funcs) - 1; i >= 0; i-- {
		if f := c.funcs[i]; f.name == e.Name && len(f.args) == len(e.Args) {
			return c.compileCallPc(f, e.Args)
		}
	}
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
					c.append(&code{op: opcallpc})
				}
				return nil
			}
		}
	}
	if e.Name[0] == '$' {
		return &variableNotFoundError{e.Name}
	}
	name := e.Name
	if name[0] == '_' {
		name = name[1:]
	}
	if q, ok := builtinFuncs[name]; ok {
		for _, fd := range q.FuncDefs {
			if len(fd.Args) == len(e.Args) {
				if err := c.compileFuncDef(fd, true); err != nil {
					return err
				}
			}
		}
		for i := len(c.funcs) - 1; i >= 0; i-- {
			if f := c.funcs[i]; f.name == e.Name && len(f.args) == len(e.Args) {
				return c.compileCallPc(f, e.Args)
			}
		}
	}
	if fn, ok := internalFuncs[e.Name]; ok && fn.accept(len(e.Args)) {
		switch e.Name {
		case "empty":
			c.append(&code{op: opbacktrack})
			return nil
		case "path":
			c.append(&code{op: oppathbegin})
			if err := c.compileCall(e.Name, e.Args); err != nil {
				return err
			}
			c.codes[len(c.codes)-1] = &code{op: oppathend}
			return nil
		default:
			return c.compileCall(e.Name, e.Args)
		}
	}
	return &funcNotFoundError{e}
}

func (c *compiler) compileObject(e *Object) error {
	c.appendCodeInfo(e)
	if len(e.KeyVals) == 0 {
		c.append(&code{op: opconst, v: map[string]interface{}{}})
		return nil
	}
	v := c.newVariable()
	c.append(&code{op: opstore, v: v})
	for _, kv := range e.KeyVals {
		if kv.KeyOnly != nil {
			if (*kv.KeyOnly)[0] == '$' {
				c.append(&code{op: oppush, v: (*kv.KeyOnly)[1:]})
				c.append(&code{op: opload, v: v})
				if err := c.compileFunc(&Func{Name: *kv.KeyOnly}); err != nil {
					return err
				}
			} else {
				c.append(&code{op: oppush, v: *kv.KeyOnly})
				c.append(&code{op: opload, v: v})
				if err := c.compileIndex(&Term{Identity: true}, &Index{Name: *kv.KeyOnly}); err != nil {
					return err
				}
			}
		} else if kv.KeyOnlyString != "" {
			c.append(&code{op: opload, v: v})
			if err := c.compileString(kv.KeyOnlyString); err != nil {
				return err
			}
			c.append(&code{op: opdup})
			c.append(&code{op: opload, v: v})
			c.append(&code{op: opload, v: v})
			// ref: compileCall
			c.append(&code{op: opcall, v: [3]interface{}{internalFuncs["_index"].callback, 2, "_index"}})
		} else {
			if kv.Pipe != nil {
				c.append(&code{op: opload, v: v})
				if err := c.compilePipe(kv.Pipe); err != nil {
					return err
				}
			} else if kv.KeyString != "" {
				c.append(&code{op: opload, v: v})
				if err := c.compileString(kv.KeyString); err != nil {
					return err
				}
			} else {
				c.append(&code{op: oppush, v: kv.Key})
			}
			c.append(&code{op: opload, v: v})
			if err := c.compileExpr(kv.Val); err != nil {
				return err
			}
		}
	}
	c.append(&code{op: opobject, v: len(e.KeyVals)})
	return nil
}

func (c *compiler) compileArray(e *Array) error {
	c.appendCodeInfo(e)
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

func (c *compiler) compileUnary(e *Unary) error {
	c.appendCodeInfo(e)
	if err := c.compileTerm(e.Term); err != nil {
		return err
	}
	switch e.Op {
	case OpAdd:
		return c.compileCall("_plus", nil)
	case OpSub:
		return c.compileCall("_negate", nil)
	default:
		return fmt.Errorf("unexpected operator in Unary: %s", e.Op)
	}
}

func (c *compiler) compileString(s string) error {
	if !strings.Contains(s, "\\(") {
		c.append(&code{op: opconst, v: s[1 : len(s)-1]})
		return nil
	}
	p, err := c.stringToPipe(s)
	if err != nil {
		return err
	}
	return c.compilePipe(p)
}

func (c *compiler) stringToPipe(s string) (*Pipe, error) {
	// ref: strconv.Unquote
	x := s[1 : len(s)-1]
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(x)/2)
	var xs []*Alt
	var es []*Expr
	var cnt int
	for len(x) > 0 {
		r, multibyte, ss, err := strconv.UnquoteChar(x, '"')
		if err != nil {
			if !strings.HasPrefix(x, "\\(") {
				return nil, err
			}
			x = x[2:]
			i, d, b := 0, 1, true
			for ; i < len(x) && b; i++ {
				switch x[i] {
				case '(':
					d++
				case ')':
					d--
					b = d != 0
				}
			}
			if i == len(x) && b {
				return nil, &stringLiteralError{s}
			}
			q, err := Parse(x[:i-1])
			if err != nil {
				return nil, err
			}
			x = x[i:]
			if len(buf) > 0 {
				xs = append(xs, (&Term{RawStr: string(buf)}).toAlt())
				buf = buf[:0]
			}
			if p := q.Pipe; p != nil {
				p.Commas = append(
					p.Commas,
					(&Term{Func: &Func{Name: "tostring"}}).toPipe().Commas...,
				)
				name := fmt.Sprintf("$%%%d", cnt)
				es = append(es, &Expr{
					Logic: (&Term{Pipe: p}).toLogic(),
					Bind:  &ExprBind{Pattern: &Pattern{Name: name}},
				})
				xs = append(xs, (&Term{Func: &Func{Name: name}}).toAlt())
				cnt++
			}
			continue
		}
		x = ss
		if r < utf8.RuneSelf || !multibyte {
			buf = append(buf, byte(r))
		} else {
			n := utf8.EncodeRune(runeTmp[:], r)
			buf = append(buf, runeTmp[:n]...)
		}
	}
	if len(xs) == 0 {
		return (&Term{RawStr: string(buf)}).toPipe(), nil
	}
	if len(buf) > 0 {
		xs = append(xs, (&Term{RawStr: string(buf)}).toAlt())
	}
	p := (&Term{Array: &Array{&Pipe{Commas: []*Comma{&Comma{Alts: xs}}}}}).toPipe()
	p.Commas = append(p.Commas, (&Term{
		Func: &Func{
			Name: "join",
			Args: []*Pipe{(&Term{Str: `""`}).toPipe()},
		}}).toPipe().Commas...)
	for _, e := range es {
		e.Bind.Body, p = p, e.toPipe()
	}
	c.appendCodeInfo(p)
	return p, nil
}

func (c *compiler) compileTermSuffix(e *Term, s *Suffix) error {
	if s.Index != nil {
		return c.compileIndex(e, s.Index)
	} else if s.SuffixIndex != nil {
		return c.compileIndex(e, s.SuffixIndex.toIndex())
	} else if s.Iter {
		if err := c.compileTerm(e); err != nil {
			return err
		}
		c.append(&code{op: opeach})
		return nil
	} else if s.Optional {
		if len(e.SuffixList) > 1 || len(e.SuffixList) == 1 && !e.SuffixList[0].Iter {
			if u, ok := e.SuffixList[len(e.SuffixList)-1].toTerm(); ok {
				t := *e // clone without changing e
				(&t).SuffixList = t.SuffixList[:len(e.SuffixList)-1]
				if err := c.compileTerm(&t); err != nil {
					return err
				}
				return c.compileTermSuffix(u, s)
			}
		}
		return c.compileTry(&Try{Body: e.toPipe()})
	} else {
		return fmt.Errorf("invalid suffix: %s", s)
	}
}

func (c *compiler) compileCall(name string, args []*Pipe) error {
	return c.compileCallInternal(
		[3]interface{}{internalFuncs[name].callback, len(args), name},
		args,
		nil,
	)
}

func (c *compiler) compileCallPc(fn *funcinfo, args []*Pipe) error {
	if len(args) == 0 {
		return c.compileCallInternal(fn.pc, args, nil)
	}
	xs, vars := make([]*Pipe, len(args)), make(map[int]bool, len(fn.args))
	for i, j := range fn.argsorder {
		xs[i] = args[j]
		if fn.args[j][0] == '$' {
			vars[i] = true
		}
	}
	return c.compileCallInternal(fn.pc, xs, vars)
}

func (c *compiler) compileCallInternal(fn interface{}, args []*Pipe, vars map[int]bool) error {
	if len(args) == 0 {
		c.append(&code{op: opcall, v: fn})
		return nil
	}
	idx := c.newVariable()
	c.append(&code{op: opstore, v: idx})
	for i := len(args) - 1; i >= 0; i-- {
		pc := c.pc() + 1 // ref: compileFuncDef
		name := fmt.Sprintf("lambda:%d", pc)
		if err := c.compileFuncDef(&FuncDef{
			Name: name,
			Body: &Query{Pipe: args[i]},
		}, false); err != nil {
			return err
		}
		if vars == nil || vars[i] {
			if pc == c.pc()-2 {
				// optimize identity argument
				j := len(c.codes) - 3
				c.codes[j] = &code{op: opload, v: idx}
				c.codes = c.codes[:j+1]
				c.funcs = c.funcs[:len(c.funcs)-1]
				c.deleteCodeInfo(name)
			} else if pc == c.pc()-3 {
				// optimize one instruction argument
				j := len(c.codes) - 4
				if c.codes[j+2].op == opconst {
					c.codes[j] = &code{op: oppush, v: c.codes[j+2].v}
					c.codes = c.codes[:j+1]
				} else {
					c.codes[j] = &code{op: opload, v: idx}
					c.codes[j+1] = c.codes[j+2]
					c.codes = c.codes[:j+2]
				}
				c.funcs = c.funcs[:len(c.funcs)-1]
				c.deleteCodeInfo(name)
			} else {
				c.append(&code{op: opload, v: idx})
				c.append(&code{op: oppushpc, v: pc})
				c.append(&code{op: opcallpc})
			}
		} else {
			c.append(&code{op: oppushpc, v: pc})
		}
	}
	c.append(&code{op: opload, v: idx})
	c.append(&code{op: opcall, v: fn})
	return nil
}

func (c *compiler) append(code *code) {
	c.codes = append(c.codes, code)
}

func (c *compiler) pc() int {
	return c.codeoffset + len(c.codes)
}

func (c *compiler) lazy(f func() *code) func() {
	i := len(c.codes)
	c.codes = append(c.codes, &code{op: opnop})
	return func() { c.codes[i] = f() }
}

func (c *compiler) optimizeTailRec() {
	var pcs []int
	targets := map[int]bool{}
	for i := 0; i < len(c.codes); i++ {
		switch c.codes[i].op {
		case opscope:
			pc := i + c.codeoffset
			pcs = append(pcs, pc)
			xs := c.codes[i].v.([2]int)
			if xs[1] == 0 {
				targets[pc] = true
			}
		case opcall:
			if j, ok := c.codes[i].v.(int); !ok || pcs[len(pcs)-1] != j || !targets[j] {
				break
			}
		loop:
			for j := i + 1; j < len(c.codes); {
				switch c.codes[j].op {
				case opjump:
					j = c.codes[j].v.(int) - c.codeoffset
				case opret:
					c.codes[i] = &code{op: opjump, v: pcs[len(pcs)-1] + 1}
					break loop
				default:
					break loop
				}
			}
		case opret:
			pcs = pcs[:len(pcs)-1]
		}
	}
}

func (c *compiler) optimizeJumps() {
	for i := len(c.codes) - 1; i >= 0; i-- {
		code := c.codes[i]
		if code.op != opjump {
			continue
		}
		if code.v.(int)-1 == i+c.codeoffset {
			c.codes[i].op = opnop
			continue
		}
		for {
			d := c.codes[code.v.(int)-c.codeoffset]
			if d.op != opjump {
				break
			}
			code.v = d.v
		}
	}
}
