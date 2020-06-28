package gojq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type compiler struct {
	moduleLoader  ModuleLoader
	environLoader func() []string
	variables     []string
	inputIter     Iter
	codes         []*code
	codeinfos     []codeinfo
	codeoffset    int
	scopes        []*scopeinfo
	scopecnt      int
	funcs         []*funcinfo
}

// Code is a compiled jq query.
type Code struct {
	variables []string
	codes     []*code
	codeinfos []codeinfo
}

// Run runs the code with the variable values (which should be in the
// same order as the given variables using WithVariables) and returns
// a result iterator.
func (c *Code) Run(v interface{}, values ...interface{}) Iter {
	return c.RunWithContext(nil, v, values...)
}

// RunWithContext runs the code with context.
func (c *Code) RunWithContext(ctx context.Context, v interface{}, values ...interface{}) Iter {
	if len(values) > len(c.variables) {
		return unitIterator(errTooManyVariableValues)
	} else if len(values) < len(c.variables) {
		return unitIterator(&expectedVariableError{c.variables[len(values)]})
	}
	for i, v := range values {
		values[i] = normalizeNumbers(v)
	}
	return newEnv(ctx).execute(c, normalizeNumbers(v), values...)
}

// ModuleLoader is an interface for loading modules.
type ModuleLoader interface {
	LoadModule(string) (*Module, error)
	// (optional) LoadModuleWithMeta(string, map[string]interface{}) (*Module, error)
	// (optional) LoadInitModules() ([]*Module, error)
	// (optional) LoadJSON(string) (interface{}, error)
	// (optional) LoadJSONWithMeta(string, map[string]interface{}) (interface{}, error)
}

type codeinfo struct {
	name string
	pc   int
}

type scopeinfo struct {
	id          int
	depth       int
	variables   []varinfo
	variablecnt int
}

type varinfo struct {
	name  string
	index [2]int
	depth int
}

type funcinfo struct {
	name      string
	pc        int
	args      []string
	argsorder []int
}

// Compile compiles a query.
func Compile(q *Query, options ...CompilerOption) (*Code, error) {
	c := &compiler{}
	for _, opt := range options {
		opt(c)
	}
	scope := c.newScope()
	c.scopes = []*scopeinfo{scope}
	defer c.lazy(func() *code {
		return &code{op: opscope, v: [2]int{scope.id, scope.variablecnt}}
	})()
	if c.moduleLoader != nil {
		if moduleLoader, ok := c.moduleLoader.(interface {
			LoadInitModules() ([]*Module, error)
		}); ok {
			ms, err := moduleLoader.LoadInitModules()
			if err != nil {
				return nil, err
			}
			for _, m := range ms {
				if err := c.compileModule(m, ""); err != nil {
					return nil, err
				}
			}
		}
	}
	code, err := c.compile(q)
	if err != nil {
		return nil, err
	}
	c.optimizeJumps()
	return code, nil
}

var varNameRe = regexp.MustCompile(`^\$[a-zA-Z_][a-zA-Z0-9_]*$`)

func (c *compiler) compile(q *Query) (*Code, error) {
	for _, name := range c.variables {
		if !varNameRe.MatchString(name) {
			return nil, &variableNameError{name}
		}
		v := c.pushVariable(name)
		c.append(&code{op: opstore, v: v})
	}
	for _, i := range q.Imports {
		if err := c.compileImport(i); err != nil {
			return nil, err
		}
	}
	if err := c.compileQuery(q); err != nil {
		return nil, err
	}
	c.append(&code{op: opret})
	return &Code{
		variables: c.variables,
		codes:     c.codes,
		codeinfos: c.codeinfos,
	}, nil
}

func (c *compiler) compileImport(i *Import) error {
	var path, alias string
	if i.ImportPath != "" {
		path, alias = i.ImportPath, i.ImportAlias
	} else {
		path = i.IncludePath
	}
	if c.moduleLoader == nil {
		return fmt.Errorf("cannot load module: %s", path)
	}
	path, err := strconv.Unquote(path)
	if err != nil {
		return err
	}
	if strings.HasPrefix(alias, "$") {
		var vals interface{}
		if moduleLoader, ok := c.moduleLoader.(interface {
			LoadJSONWithMeta(string, map[string]interface{}) (interface{}, error)
		}); ok {
			if vals, err = moduleLoader.LoadJSONWithMeta(path, i.Meta.ToValue()); err != nil {
				return err
			}
		} else if moduleLoader, ok := c.moduleLoader.(interface {
			LoadJSON(string) (interface{}, error)
		}); ok {
			if vals, err = moduleLoader.LoadJSON(path); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("module not found: %q", path)
		}
		c.append(&code{op: oppush, v: vals})
		c.append(&code{op: opstore, v: c.pushVariable(alias)})
		c.append(&code{op: oppush, v: vals})
		c.append(&code{op: opstore, v: c.pushVariable(alias + "::" + alias[1:])})
		return nil
	}
	var m *Module
	if moduleLoader, ok := c.moduleLoader.(interface {
		LoadModuleWithMeta(string, map[string]interface{}) (*Module, error)
	}); ok {
		if m, err = moduleLoader.LoadModuleWithMeta(path, i.Meta.ToValue()); err != nil {
			return err
		}
	} else if m, err = c.moduleLoader.LoadModule(path); err != nil {
		return err
	}
	c.appendCodeInfo("module " + path)
	defer c.appendCodeInfo("end of module " + path)
	return c.compileModule(m, alias)
}

func (c *compiler) compileModule(m *Module, alias string) error {
	cc := &compiler{
		moduleLoader: c.moduleLoader, environLoader: c.environLoader, variables: c.variables, inputIter: c.inputIter,
		codeoffset: c.pc(), scopes: c.scopes, scopecnt: c.scopecnt}
	defer cc.newScopeDepth()()
	bs, err := cc.compileModuleInternal(m)
	if err != nil {
		return err
	}
	c.codes = append(c.codes, bs.codes...)
	if alias != "" {
		for _, f := range cc.funcs {
			f.name = alias + "::" + f.name
		}
	}
	c.funcs = append(c.funcs, cc.funcs...)
	c.codeinfos = append(c.codeinfos, bs.codeinfos...)
	c.scopecnt = cc.scopecnt
	return nil
}

func (c *compiler) compileModuleInternal(m *Module) (*Code, error) {
	for _, i := range m.Imports {
		if err := c.compileImport(i); err != nil {
			return nil, err
		}
	}
	for _, fd := range m.FuncDefs {
		if err := c.compileFuncDef(fd, false); err != nil {
			return nil, err
		}
	}
	return &Code{codes: c.codes, codeinfos: c.codeinfos}, nil
}

func (c *compiler) newVariable() [2]int {
	return c.pushVariable("")
}

func (c *compiler) pushVariable(name string) [2]int {
	s := c.scopes[len(c.scopes)-1]
	if name != "" {
		for _, v := range s.variables {
			if v.name == name && v.depth == s.depth {
				return v.index
			}
		}
	}
	v := [2]int{s.id, s.variablecnt}
	s.variablecnt++
	s.variables = append(s.variables, varinfo{name, v, s.depth})
	return v
}

func (c *compiler) newScope() *scopeinfo {
	i := c.scopecnt // do not use len(c.scopes) because it pops
	c.scopecnt++
	return &scopeinfo{i, 0, nil, 0}
}

func (c *compiler) newScopeDepth() func() {
	scope := c.scopes[len(c.scopes)-1]
	l, m := len(scope.variables), len(c.funcs)
	scope.depth++
	return func() {
		scope.depth--
		scope.variables = scope.variables[:l]
		c.funcs = c.funcs[:m]
	}
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
	defer c.appendCodeInfo("end of " + e.Name)
	pc, argsorder := c.pc(), getArgsOrder(e.Args)
	c.funcs = append(c.funcs, &funcinfo{e.Name, pc, e.Args, argsorder})
	cc := &compiler{
		moduleLoader: c.moduleLoader, environLoader: c.environLoader, inputIter: c.inputIter,
		codeoffset: pc, scopecnt: c.scopecnt, funcs: c.funcs}
	scope := cc.newScope()
	cc.scopes = append(c.scopes, scope)
	setscope := cc.lazy(func() *code {
		return &code{op: opscope, v: [2]int{scope.id, scope.variablecnt}}
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

func (c *compiler) compileQuery(e *Query) error {
	for _, e := range e.Commas {
		if err := c.compileComma(e); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileComma(e *Comma) error {
	if e.Func != "" { // ref: Term#toFunc
		switch e.Func {
		case ".":
			return c.compileTerm(&Term{Identity: true})
		case "..":
			return c.compileTerm(&Term{Recurse: true})
		case "null":
			return c.compileTerm(&Term{Null: true})
		case "true":
			return c.compileTerm(&Term{True: true})
		case "false":
			return c.compileTerm(&Term{False: true})
		default:
			return c.compileFunc(&Func{Name: e.Func})
		}
	} else if len(e.Filters) == 1 {
		return c.compileFilter(e.Filters[0])
	}
	setfork := c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() + 1}
	})
	if err := c.compileComma(&Comma{Filters: e.Filters[:len(e.Filters)-1]}); err != nil {
		return err
	}
	setfork()
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()}
	})()
	return c.compileFilter(e.Filters[len(e.Filters)-1])
}

func (c *compiler) compileFilter(e *Filter) error {
	for _, fd := range e.FuncDefs {
		if err := c.compileFuncDef(fd, false); err != nil {
			return err
		}
	}
	return c.compileAlt(e.Alt)
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
		return c.compileExprUpdate(e)
	} else if e.Bind != nil {
		c.append(&code{op: opdup})
		c.append(&code{op: opexpbegin})
		defer func() {
			if err == nil {
				err = c.compileBind(e.Bind)
			}
		}()
	}
	if e.Logic != nil {
		return c.compileLogic(e.Logic)
	} else if e.Label != nil {
		return c.compileLabel(e.Label)
	} else {
		return fmt.Errorf("invalid expr: %s", e)
	}
}

func (c *compiler) compileExprUpdate(e *Expr) (err error) {
	t := *e // clone without changing e
	(&t).Update = nil
	switch e.UpdateOp {
	case OpAssign:
		// .foo.bar = f => setpath(["foo", "bar"]; f)
		if xs := t.toIndices(); xs != nil {
			// ref: compileCall
			v := c.newVariable()
			c.append(&code{op: opstore, v: v})
			c.append(&code{op: opload, v: v})
			if err := c.compileAlt(e.Update); err != nil {
				return err
			}
			c.append(&code{op: oppush, v: xs})
			c.append(&code{op: opload, v: v})
			c.append(&code{op: opcall, v: [3]interface{}{internalFuncs["setpath"].callback, 2, "setpath"}})
			return nil
		}
		fallthrough
	case OpModify:
		return c.compileFunc(
			&Func{
				Name: e.UpdateOp.getFunc(),
				Args: []*Query{
					t.toQuery(),
					e.Update.toQuery(),
				},
			},
		)
	default:
		name := "$%0"
		c.append(&code{op: opdup})
		if err := c.compileAlt(e.Update); err != nil {
			return err
		}
		c.append(&code{op: opstore, v: c.pushVariable(name)})
		return c.compileFunc(
			&Func{
				Name: "_modify",
				Args: []*Query{
					t.toQuery(),
					(&Term{Func: &Func{
						Name: e.UpdateOp.getFunc(),
						Args: []*Query{
							(&Term{Identity: true}).toQuery(),
							(&Term{Func: &Func{Name: name}}).toQuery(),
						},
					}}).toQuery(),
				},
			},
		)
	}
}

func (c *compiler) compileBind(b *Bind) error {
	var pc int
	var vs [][2]int
	for i, p := range b.Patterns {
		var pcc int
		var err error
		if i < len(b.Patterns)-1 {
			defer c.lazy(func() *code {
				return &code{op: opforkalt, v: pcc}
			})()
		}
		if 0 < i {
			for _, v := range vs {
				c.append(&code{op: oppush, v: nil})
				c.append(&code{op: opstore, v: v})
			}
		}
		vs, err = c.compilePattern(p)
		if err != nil {
			return err
		}
		if i < len(b.Patterns)-1 {
			defer c.lazy(func() *code {
				return &code{op: opjump, v: pc}
			})()
			pcc = c.pc()
		}
	}
	if len(b.Patterns) > 1 {
		pc = c.pc()
	}
	if c.codes[len(c.codes)-2].op == opexpbegin {
		c.codes[len(c.codes)-2] = c.codes[len(c.codes)-1]
		c.codes = c.codes[:len(c.codes)-1]
	} else {
		c.append(&code{op: opexpend}) // ref: compileExpr
	}
	return c.compileQuery(b.Body)
}

func (c *compiler) compilePattern(p *Pattern) ([][2]int, error) {
	c.appendCodeInfo(p)
	if p.Name != "" {
		v := c.pushVariable(p.Name)
		c.append(&code{op: opstore, v: v})
		return [][2]int{v}, nil
	} else if len(p.Array) > 0 {
		var vs [][2]int
		v := c.newVariable()
		c.append(&code{op: opstore, v: v})
		for i, p := range p.Array {
			c.append(&code{op: oppush, v: i})
			c.append(&code{op: opload, v: v})
			c.append(&code{op: opload, v: v})
			// ref: compileCall
			c.append(&code{op: opcall, v: [3]interface{}{internalFuncs["_index"].callback, 2, "_index"}})
			ns, err := c.compilePattern(p)
			if err != nil {
				return nil, err
			}
			vs = append(vs, ns...)
		}
		return vs, nil
	} else if len(p.Object) > 0 {
		var vs [][2]int
		v := c.newVariable()
		c.append(&code{op: opstore, v: v})
		for _, kv := range p.Object {
			var key, name string
			if kv.KeyOnly != "" {
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
			} else if kv.Query != nil {
				c.append(&code{op: opload, v: v})
				if err := c.compileQuery(kv.Query); err != nil {
					return nil, err
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
				ns, err := c.compilePattern(&Pattern{Name: name})
				if err != nil {
					return nil, err
				}
				vs = append(vs, ns...)
			}
			if kv.Val != nil {
				ns, err := c.compilePattern(kv.Val)
				if err != nil {
					return nil, err
				}
				vs = append(vs, ns...)
			}
		}
		return vs, nil
	} else {
		return nil, fmt.Errorf("invalid pattern: %s", p)
	}
}

func (c *compiler) compileLogic(e *Logic) error {
	if len(e.Right) == 0 {
		return c.compileAndExpr(e.Left)
	}
	return c.compileIf(
		&If{
			Cond: (&Logic{e.Left, e.Right[:len(e.Right)-1]}).toQuery(),
			Then: (&Term{True: true}).toQuery(),
			Else: (&Term{If: &If{
				Cond: e.Right[len(e.Right)-1].Right.toQuery(),
				Then: (&Term{True: true}).toQuery(),
				Else: (&Term{False: true}).toQuery(),
			}}).toQuery(),
		},
	)
}

func (c *compiler) compileIf(e *If) error {
	c.appendCodeInfo(e)
	c.append(&code{op: opdup}) // duplicate the value for then or else clause
	c.append(&code{op: opexpbegin})
	f := c.newScopeDepth()
	if err := c.compileQuery(e.Cond); err != nil {
		return err
	}
	f()
	c.append(&code{op: opexpend})
	setjumpifnot := c.lazy(func() *code {
		return &code{op: opjumpifnot, v: c.pc() + 1} // if falsy, skip then clause
	})
	f = c.newScopeDepth()
	if err := c.compileQuery(e.Then); err != nil {
		return err
	}
	f()
	setjumpifnot()
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()} // jump to ret after else clause
	})()
	if len(e.Elif) > 0 {
		return c.compileIf(&If{e.Elif[0].Cond, e.Elif[0].Then, e.Elif[1:], e.Else})
	}
	if e.Else != nil {
		defer c.newScopeDepth()()
		return c.compileQuery(e.Else)
	}
	return nil
}

func (c *compiler) compileTry(e *Try) error {
	c.appendCodeInfo(e)
	setforktrybegin := c.lazy(func() *code {
		return &code{op: opforktrybegin, v: c.pc()}
	})
	f := c.newScopeDepth()
	if err := c.compileQuery(e.Body); err != nil {
		return err
	}
	f()
	c.append(&code{op: opforktryend})
	defer c.lazy(func() *code {
		return &code{op: opjump, v: c.pc()}
	})()
	setforktrybegin()
	if e.Catch != nil {
		defer c.newScopeDepth()()
		return c.compileTerm(e.Catch)
	}
	c.append(&code{op: opbacktrack})
	return nil
}

func (c *compiler) compileReduce(e *Reduce) error {
	c.appendCodeInfo(e)
	defer c.newScopeDepth()()
	defer c.lazy(func() *code {
		return &code{op: opfork, v: c.pc() - 2}
	})()
	c.append(&code{op: opdup})
	v := c.newVariable()
	f := c.newScopeDepth()
	if err := c.compileQuery(e.Start); err != nil {
		return err
	}
	f()
	c.append(&code{op: opstore, v: v})
	if err := c.compileTerm(e.Term); err != nil {
		return err
	}
	if _, err := c.compilePattern(e.Pattern); err != nil {
		return err
	}
	c.append(&code{op: opload, v: v})
	f = c.newScopeDepth()
	if err := c.compileQuery(e.Update); err != nil {
		return err
	}
	f()
	c.append(&code{op: opstore, v: v})
	c.append(&code{op: opbacktrack})
	c.append(&code{op: oppop})
	c.append(&code{op: opload, v: v})
	return nil
}

func (c *compiler) compileForeach(e *Foreach) error {
	c.appendCodeInfo(e)
	defer c.newScopeDepth()()
	c.append(&code{op: opdup})
	v := c.newVariable()
	f := c.newScopeDepth()
	if err := c.compileQuery(e.Start); err != nil {
		return err
	}
	f()
	c.append(&code{op: opstore, v: v})
	if err := c.compileTerm(e.Term); err != nil {
		return err
	}
	if _, err := c.compilePattern(e.Pattern); err != nil {
		return err
	}
	c.append(&code{op: opload, v: v})
	f = c.newScopeDepth()
	if err := c.compileQuery(e.Update); err != nil {
		return err
	}
	f()
	c.append(&code{op: opdup})
	c.append(&code{op: opstore, v: v})
	if e.Extract != nil {
		defer c.newScopeDepth()()
		return c.compileQuery(e.Extract)
	}
	return nil
}

func (c *compiler) compileLabel(e *Label) error {
	c.appendCodeInfo(e)
	defer c.lazy(func() *code {
		return &code{op: opforklabel, v: e.Ident}
	})()
	return c.compileQuery(e.Body)
}

func (c *compiler) compileAndExpr(e *AndExpr) error {
	if len(e.Right) == 0 {
		return c.compileCompare(e.Left)
	}
	return c.compileIf(
		&If{
			Cond: (&AndExpr{e.Left, e.Right[:len(e.Right)-1]}).toQuery(),
			Then: (&Term{If: &If{
				Cond: e.Right[len(e.Right)-1].Right.toQuery(),
				Then: (&Term{True: true}).toQuery(),
				Else: (&Term{False: true}).toQuery(),
			}}).toQuery(),
			Else: (&Term{False: true}).toQuery(),
		},
	)
}

func (c *compiler) compileCompare(e *Compare) error {
	if e.Right == nil {
		return c.compileArith(e.Left)
	}
	return c.compileCall(
		e.Right.Op.getFunc(),
		[]*Query{e.Left.toQuery(), e.Right.Right.toQuery()},
	)
}

func (c *compiler) compileArith(e *Arith) error {
	if len(e.Right) == 0 {
		return c.compileFactor(e.Left)
	}
	r := e.Right[len(e.Right)-1]
	return c.compileCall(
		r.Op.getFunc(),
		[]*Query{
			(&Arith{e.Left, e.Right[:len(e.Right)-1]}).toQuery(),
			r.Right.toQuery(),
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
		[]*Query{
			(&Factor{e.Left, e.Right[:len(e.Right)-1]}).toQuery(),
			r.Right.toQuery(),
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
	} else if e.Null {
		c.append(&code{op: opconst, v: nil})
		return nil
	} else if e.True {
		c.append(&code{op: opconst, v: true})
		return nil
	} else if e.False {
		c.append(&code{op: opconst, v: false})
		return nil
	} else if e.Func != nil {
		return c.compileFunc(e.Func)
	} else if e.Object != nil {
		return c.compileObject(e.Object)
	} else if e.Array != nil {
		return c.compileArray(e.Array)
	} else if e.Number != "" {
		v := normalizeNumbers(json.Number(e.Number))
		if err, ok := v.(error); ok {
			return err
		}
		c.append(&code{op: opconst, v: v})
		return nil
	} else if e.Unary != nil {
		return c.compileUnary(e.Unary)
	} else if e.Format != "" {
		return c.compileFormat(e.Format, e.FormatStr)
	} else if e.Str != "" {
		return c.compileString(e.Str)
	} else if e.RawStr != "" {
		c.append(&code{op: opconst, v: e.RawStr})
		return nil
	} else if e.If != nil {
		return c.compileIf(e.If)
	} else if e.Try != nil {
		return c.compileTry(e.Try)
	} else if e.Reduce != nil {
		return c.compileReduce(e.Reduce)
	} else if e.Foreach != nil {
		return c.compileForeach(e.Foreach)
	} else if e.Break != "" {
		c.append(&code{op: opconst, v: e.Break})
		return c.compileCall("_break", nil)
	} else if e.Query != nil {
		defer c.newScopeDepth()()
		return c.compileQuery(e.Query)
	} else {
		return fmt.Errorf("invalid term: %s", e)
	}
}

func (c *compiler) compileIndex(e *Term, x *Index) error {
	c.appendCodeInfo(x)
	if x.Name != "" {
		return c.compileCall("_index", []*Query{e.toQuery(), (&Term{RawStr: x.Name[1:]}).toQuery()})
	}
	if x.Str != "" {
		q, err := c.stringToQuery(x.Str, nil)
		if err != nil {
			return err
		}
		return c.compileCall("_index", []*Query{e.toQuery(), q})
	}
	if x.Start != nil {
		if x.IsSlice {
			if x.End != nil {
				return c.compileCall("_slice", []*Query{e.toQuery(), x.End, x.Start})
			}
			return c.compileCall("_slice", []*Query{e.toQuery(), (&Term{Null: true}).toQuery(), x.Start})
		}
		return c.compileCall("_index", []*Query{e.toQuery(), x.Start})
	}
	return c.compileCall("_slice", []*Query{e.toQuery(), x.End, (&Term{Null: true}).toQuery()})
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
	if (e.Name == "$ENV" || e.Name == "env") && len(e.Args) == 0 {
		env := make(map[string]interface{})
		if c.environLoader != nil {
			for _, kv := range c.environLoader() {
				xs := strings.SplitN(kv, "=", 2)
				env[xs[0]] = xs[1]
			}
		}
		c.append(&code{op: opconst, v: env})
		return nil
	}
	if e.Name[0] == '$' {
		return &variableNotFoundError{e.Name}
	}
	name := e.Name
	if name[0] == '_' {
		name = name[1:]
	}
	if fds, ok := builtinFuncDefs[name]; ok {
		for _, fd := range fds {
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
		case "debug":
			c.append(&code{op: opdebug, v: "DEBUG:"})
			return nil
		case "stderr":
			c.append(&code{op: opdebug, v: "STDERR:"})
			return nil
		case "input":
			if c.inputIter == nil {
				return &inputNotAllowedError{}
			}
			return c.compileCallInternal(
				[3]interface{}{c.funcInput, 0, e.Name},
				e.Args,
				nil,
				false,
			)
		case "modulemeta":
			return c.compileCallInternal(
				[3]interface{}{c.funcModulemeta, 0, e.Name},
				e.Args,
				nil,
				false,
			)
		default:
			return c.compileCall(e.Name, e.Args)
		}
	}
	return &funcNotFoundError{e}
}

func (c *compiler) funcInput(interface{}, []interface{}) interface{} {
	v, ok := c.inputIter.Next()
	if !ok {
		return errors.New("break")
	}
	return normalizeNumbers(v)
}

func (c *compiler) funcModulemeta(v interface{}, _ []interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return &funcTypeError{"modulemeta", v}
	}
	if c.moduleLoader == nil {
		return fmt.Errorf("cannot load module: %q", s)
	}
	var m *Module
	var err error
	if moduleLoader, ok := c.moduleLoader.(interface {
		LoadModuleWithMeta(string, map[string]interface{}) (*Module, error)
	}); ok {
		if m, err = moduleLoader.LoadModuleWithMeta(s, nil); err != nil {
			return err
		}
	} else if m, err = c.moduleLoader.LoadModule(s); err != nil {
		return err
	}
	meta := m.Meta.ToValue()
	if meta == nil {
		meta = make(map[string]interface{})
	}
	var deps []interface{}
	for _, i := range m.Imports {
		v := i.Meta.ToValue()
		if v == nil {
			v = make(map[string]interface{})
		} else {
			for k := range v {
				// dirty hack to remove the extra fields added in the cli package
				if strings.HasPrefix(k, "$$") {
					delete(v, k)
				}
			}
		}
		if i.ImportPath == "" {
			v["relpath"], err = strconv.Unquote(i.IncludePath)
		} else {
			v["relpath"], err = strconv.Unquote(i.ImportPath)
		}
		if err != nil {
			return err
		}
		if i.ImportAlias != "" {
			v["as"] = strings.TrimPrefix(i.ImportAlias, "$")
		}
		v["is_data"] = strings.HasPrefix(i.ImportAlias, "$")
		deps = append(deps, v)
	}
	meta["deps"] = deps
	return meta
}

func (c *compiler) compileObject(e *Object) error {
	c.appendCodeInfo(e)
	if len(e.KeyVals) == 0 {
		c.append(&code{op: opconst, v: map[string]interface{}{}})
		return nil
	}
	defer c.newScopeDepth()()
	v := c.newVariable()
	c.append(&code{op: opstore, v: v})
	pc := len(c.codes)
	for _, kv := range e.KeyVals {
		if err := c.compileObjectKeyVal(v, kv); err != nil {
			return err
		}
	}
	c.append(&code{op: opobject, v: len(e.KeyVals)})
	// optimize constant objects
	l := len(e.KeyVals)
	if pc+l*3+1 != len(c.codes) {
		return nil
	}
	for i := 0; i < l; i++ {
		if c.codes[pc+i*3].op != oppush ||
			c.codes[pc+i*3+1].op != opload ||
			c.codes[pc+i*3+2].op != opconst {
			return nil
		}
	}
	w := make(map[string]interface{}, l)
	for i := 0; i < l; i++ {
		w[c.codes[pc+i*3].v.(string)] = c.codes[pc+i*3+2].v
	}
	c.codes[pc-1] = &code{op: opconst, v: w}
	c.codes = c.codes[:pc]
	return nil
}

func (c *compiler) compileObjectKeyVal(v [2]int, kv ObjectKeyVal) error {
	if kv.KeyOnly != nil {
		if (*kv.KeyOnly)[0] == '$' {
			c.append(&code{op: oppush, v: (*kv.KeyOnly)[1:]})
			c.append(&code{op: opload, v: v})
			return c.compileFunc(&Func{Name: *kv.KeyOnly})
		}
		c.append(&code{op: oppush, v: *kv.KeyOnly})
		c.append(&code{op: opload, v: v})
		return c.compileIndex(&Term{Identity: true}, &Index{Name: "." + *kv.KeyOnly})
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
		return nil
	} else {
		if kv.Query != nil {
			c.append(&code{op: opload, v: v})
			f := c.newScopeDepth()
			if err := c.compileQuery(kv.Query); err != nil {
				return err
			}
			f()
		} else if kv.KeyString != "" {
			c.append(&code{op: opload, v: v})
			if err := c.compileString(kv.KeyString); err != nil {
				return err
			}
		} else if kv.Key[0] == '$' {
			c.append(&code{op: opload, v: v})
			if err := c.compileFunc(&Func{Name: kv.Key}); err != nil {
				return err
			}
		} else {
			c.append(&code{op: oppush, v: kv.Key})
		}
		c.append(&code{op: opload, v: v})
		return c.compileObjectVal(kv.Val)
	}
}

func (c *compiler) compileObjectVal(e *ObjectVal) error {
	for _, e := range e.Alts {
		if err := c.compileAlt(e); err != nil {
			return err
		}
	}
	return nil
}

func (c *compiler) compileArray(e *Array) error {
	c.appendCodeInfo(e)
	if e.Query == nil {
		c.append(&code{op: opconst, v: []interface{}{}})
		return nil
	}
	c.append(&code{op: oppush, v: []interface{}{}})
	arr := c.newVariable()
	c.append(&code{op: opstore, v: arr})
	pc := len(c.codes)
	c.append(&code{op: opnop})
	defer func() {
		if pc < len(c.codes) {
			c.codes[pc] = &code{op: opfork, v: c.pc() - 2}
		}
	}()
	defer c.newScopeDepth()()
	if err := c.compileQuery(e.Query); err != nil {
		return err
	}
	c.append(&code{op: opappend, v: arr})
	c.append(&code{op: opbacktrack})
	c.append(&code{op: oppop})
	c.append(&code{op: opload, v: arr})
	if len(e.Query.Commas) != 1 || 3*len(e.Query.Commas[0].Filters) != len(c.codes)-pc-3 {
		return nil
	}
	// optimize constant arrays
	l := len(e.Query.Commas[0].Filters)
	for i := 0; i < l; i++ {
		if (i > 0 && c.codes[pc+i].op != opfork) ||
			c.codes[pc+i*2+l].op != opconst ||
			(i < l-1 && c.codes[pc+i*2+l+1].op != opjump) {
			return nil
		}
	}
	v := make([]interface{}, l)
	for i := 0; i < l; i++ {
		v[i] = c.codes[pc+i*2+l].v
	}
	c.codes[pc-2] = &code{op: opconst, v: v}
	c.codes = c.codes[:pc-1]
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

func (c *compiler) compileFormat(fmt, str string) error {
	if f := formatToFunc(fmt); f != nil {
		if str == "" {
			return c.compileFunc(f)
		}
		q, err := c.stringToQuery(str, f)
		if err != nil {
			return err
		}
		return c.compileQuery(q)
	}
	return &formatNotFoundError{fmt}
}

func formatToFunc(fmt string) *Func {
	switch fmt {
	case "@text":
		return &Func{Name: "tostring"}
	case "@json":
		return &Func{Name: "tojson"}
	case "@html":
		return &Func{Name: "_tohtml"}
	case "@uri":
		return &Func{Name: "_touri"}
	case "@csv":
		return &Func{Name: "_tocsv"}
	case "@tsv":
		return &Func{Name: "_totsv"}
	case "@sh":
		return &Func{Name: "_tosh"}
	case "@base64":
		return &Func{Name: "_tobase64"}
	case "@base64d":
		return &Func{Name: "_tobase64d"}
	default:
		return nil
	}
}

func (c *compiler) compileString(s string) error {
	if !strings.Contains(s, "\\(") {
		s, err := strconv.Unquote(s)
		if err == nil {
			c.append(&code{op: opconst, v: s})
			return nil
		}
	}
	q, err := c.stringToQuery(s, nil)
	if err != nil {
		return err
	}
	return c.compileQuery(q)
}

func (c *compiler) stringToQuery(s string, f *Func) (*Query, error) {
	if f == nil {
		f = &Func{Name: "tostring"}
	}
	// ref: strconv.Unquote
	x := s[1 : len(s)-1]
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(x)/2)
	var xs []*Filter
	var es []*Expr
	var cnt int
	for len(x) > 0 {
		r, multibyte, ss, err := strconv.UnquoteChar(x, '"')
		if err != nil {
			if !strings.HasPrefix(x, "\\(") {
				return nil, err
			}
			match := queryInStringPattern.FindString(x)
			i := len([]byte(match))
			if i == 0 {
				return nil, &stringLiteralError{s}
			}
			t := x[2 : i-1]
			q, err := Parse(t)
			if err != nil {
				return nil, &stringQueryError{t, err}
			}
			x = x[i:]
			if len(buf) > 0 {
				xs = append(xs, (&Term{RawStr: string(buf)}).toFilter())
				buf = buf[:0]
			}
			q.Commas = append(q.Commas, (&Term{Func: f}).toQuery().Commas...)
			name := fmt.Sprintf("$%%%d", cnt)
			es = append(es, &Expr{
				Logic: (&Term{Query: q}).toLogic(),
				Bind:  &Bind{Patterns: []*Pattern{{Name: name}}},
			})
			xs = append(xs, (&Term{Func: &Func{Name: name}}).toFilter())
			cnt++
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
		return (&Term{RawStr: string(buf)}).toQuery(), nil
	}
	if len(buf) > 0 {
		xs = append(xs, (&Term{RawStr: string(buf)}).toFilter())
	}
	q := (&Term{Array: &Array{&Query{Commas: []*Comma{&Comma{Filters: xs}}}}}).toQuery()
	q.Commas = append(q.Commas, (&Term{
		Func: &Func{
			Name: "join",
			Args: []*Query{(&Term{Str: `""`}).toQuery()},
		}}).toQuery().Commas...)
	for _, e := range es {
		e.Bind.Body, q = q, e.toQuery()
	}
	c.appendCodeInfo(q)
	return q, nil
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
		return c.compileTry(&Try{Body: e.toQuery()})
	} else {
		return fmt.Errorf("invalid suffix: %s", s)
	}
}

func (c *compiler) compileCall(name string, args []*Query) error {
	return c.compileCallInternal(
		[3]interface{}{internalFuncs[name].callback, len(args), name},
		args,
		nil,
		name == "_index" || name == "_slice",
	)
}

func (c *compiler) compileCallPc(fn *funcinfo, args []*Query) error {
	if len(args) == 0 {
		return c.compileCallInternal(fn.pc, args, nil, false)
	}
	xs, vars := make([]*Query, len(args)), make(map[int]bool, len(fn.args))
	for i, j := range fn.argsorder {
		xs[i] = args[j]
		if fn.args[j][0] == '$' {
			vars[i] = true
		}
	}
	return c.compileCallInternal(fn.pc, xs, vars, false)
}

func (c *compiler) compileCallInternal(fn interface{}, args []*Query, vars map[int]bool, indexing bool) error {
	if len(args) == 0 {
		c.append(&code{op: opcall, v: fn})
		return nil
	}
	idx := c.newVariable()
	c.append(&code{op: opstore, v: idx})
	if indexing && len(args) > 1 {
		c.append(&code{op: opexpbegin})
	}
	for i := len(args) - 1; i >= 0; i-- {
		pc := c.pc() + 1 // ref: compileFuncDef
		name := fmt.Sprintf("lambda:%d", pc)
		if err := c.compileFuncDef(&FuncDef{Name: name, Body: args[i]}, false); err != nil {
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
		if indexing && i == 1 {
			if c.codes[len(c.codes)-2].op == opexpbegin {
				c.codes[len(c.codes)-2] = c.codes[len(c.codes)-1]
				c.codes = c.codes[:len(c.codes)-1]
			} else {
				c.append(&code{op: opexpend})
			}
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
			if d.op != opjump || code.v.(int) == d.v.(int) {
				break
			}
			code.v = d.v
		}
	}
}
