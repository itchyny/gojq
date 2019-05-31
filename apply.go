package gojq

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

func (env *env) applyQuery(query *Query, c <-chan interface{}) <-chan interface{} {
	for _, fd := range query.FuncDefs {
		env.addFuncDef(fd)
	}
	return env.applyPipe(query.Pipe, c)
}

func (env *env) applyPipe(p *Pipe, c <-chan interface{}) <-chan interface{} {
	for _, o := range p.Commas {
		c = env.applyComma(o, c)
	}
	return c
}

func (env *env) applyComma(o *Comma, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			for _, e := range o.Alts {
				for v := range env.applyAlt(e, unitIterator(v)) {
					d <- v
				}
			}
		}()
		return (<-chan interface{})(d)
	})
}

func (env *env) applyAlt(e *Alt, c <-chan interface{}) <-chan interface{} {
	if len(e.Right) == 0 {
		return env.applyExpr(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyExpr(e.Left, d())
	for _, r := range e.Right {
		w = binopIteratorAlt(w, env.applyExpr(r.Right, d()))
	}
	return w
}

func (env *env) applyExpr(e *Expr, c <-chan interface{}) <-chan interface{} {
	if e.Logic != nil {
		return env.applyLogic(e.Logic, c)
	}
	if e.If != nil {
		return env.applyIf(e.If, c)
	}
	if e.Try != nil {
		return env.applyTry(e.Try, c)
	}
	panic("unreachable expr")
}

func (env *env) applyLogic(e *Logic, c <-chan interface{}) <-chan interface{} {
	if len(e.Right) == 0 {
		return env.applyAndExpr(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyAndExpr(e.Left, d())
	for _, r := range e.Right {
		w = binopIteratorOr(w, env.applyAndExpr(r.Right, d()))
	}
	return w
}

func (env *env) applyAndExpr(e *AndExpr, c <-chan interface{}) <-chan interface{} {
	if len(e.Right) == 0 {
		return env.applyCompare(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyCompare(e.Left, d())
	for _, r := range e.Right {
		w = binopIteratorAnd(w, env.applyCompare(r.Right, d()))
	}
	return w
}

func (env *env) applyCompare(e *Compare, c <-chan interface{}) <-chan interface{} {
	if e.Right == nil {
		return env.applyArith(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyArith(e.Left, d())
	if r := e.Right; r != nil {
		w = binopIterator(w, env.applyArith(r.Right, d()), r.Op.Eval)
	}
	return w
}

func (env *env) applyArith(e *Arith, c <-chan interface{}) <-chan interface{} {
	if len(e.Right) == 0 {
		return env.applyFactor(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyFactor(e.Left, d())
	for _, r := range e.Right {
		w = binopIterator(w, env.applyFactor(r.Right, d()), r.Op.Eval)
	}
	return w
}

func (env *env) applyFactor(e *Factor, c <-chan interface{}) <-chan interface{} {
	if len(e.Right) == 0 {
		return env.applyTerm(e.Left, c)
	}
	d := reuseIterator(c)
	w := env.applyTerm(e.Left, d())
	for _, r := range e.Right {
		w = binopIterator(w, env.applyTerm(r.Right, d()), r.Op.Eval)
	}
	return w
}

func (env *env) applyTerm(t *Term, c <-chan interface{}) <-chan interface{} {
	if t.Bind == nil {
		return env.applyTermInternal(t, c)
	}
	if t.Bind.Pattern.Name != "" && t.Bind.Pattern.Name[0] != '$' {
		return unitIterator(&bindVariableNameError{t.Bind.Pattern.Name})
	}
	cc := reuseIterator(c)
	return mapIterator(env.applyTermInternal(t, cc()), func(v interface{}) interface{} {
		subEnv := newEnv(env)
		if err := subEnv.applyPattern(t.Bind.Pattern, v); err != nil {
			return err
		}
		return subEnv.applyPipe(t.Bind.Body, cc())
	})
}

func (env *env) applyPattern(p *Pattern, v interface{}) error {
	if p.Name != "" {
		env.values.Store(p.Name, v)
	} else if len(p.Array) > 0 {
		if v == nil {
			v = []interface{}{}
		}
		a, ok := v.([]interface{})
		if !ok {
			return &expectedArrayError{v}
		}
		for i, pi := range p.Array {
			if i < len(a) {
				if err := env.applyPattern(pi, a[i]); err != nil {
					return err
				}
			} else {
				if err := env.applyPattern(pi, nil); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (env *env) applyTermInternal(t *Term, c <-chan interface{}) (d <-chan interface{}) {
	defer func() {
		for _, s := range t.SuffixList {
			d = env.applySuffix(s, d)
		}
	}()
	if x := t.Index; x != nil {
		return env.applyIndex(x, c)
	}
	if t.Identity {
		return c
	}
	if t.Recurse {
		return env.applyFunc(&Func{Name: "recurse"}, c)
	}
	if t.Func != nil {
		return env.applyFunc(t.Func, c)
	}
	if t.Object != nil {
		return env.applyObject(t.Object, c)
	}
	if t.Array != nil {
		return env.applyArray(t.Array, c)
	}
	if t.Number != nil {
		return unitIterator(*t.Number)
	}
	if t.Unary != nil {
		return env.applyUnary(t.Unary.Op, t.Unary.Term, c)
	}
	if t.String != nil {
		return env.applyString(*t.String, c)
	}
	return env.applyPipe(t.Pipe, c)
}

func (env *env) applyIndex(x *Index, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		switch v := v.(type) {
		case nil:
			return nil
		case map[string]interface{}:
			return env.applyObjectIndex(x, v)
		case []interface{}:
			return env.applyArrayIndex(x, v)
		case string:
			switch v := env.applyArrayIndex(x, explode(v)).(type) {
			case <-chan interface{}:
				return mapIterator(v, func(v interface{}) interface{} {
					return implode(v.([]interface{}))
				})
			default:
				return v
			}
		default:
			if indexIsForObject(x) {
				return &expectedObjectError{v}
			}
			return &expectedArrayError{v}
		}
	})
}

func (env *env) applyObjectIndex(x *Index, m map[string]interface{}) interface{} {
	if !indexIsForObject(x) {
		return &expectedArrayError{m}
	}
	if x.Name != "" {
		return m[x.Name]
	}
	if x.String != nil {
		return mapIterator(env.applyString(*x.String, unitIterator(m)), func(s interface{}) interface{} {
			key, ok := s.(string)
			if !ok {
				return &objectKeyNotStringError{s}
			}
			return m[key]
		})
	}
	return mapIterator(env.applyPipe(x.Start, unitIterator(m)), func(s interface{}) interface{} {
		key, ok := s.(string)
		if !ok {
			return &objectKeyNotStringError{s}
		}
		return m[key]
	})
}

func (env *env) applyArrayIndex(x *Index, a []interface{}) interface{} {
	if x.Name != "" {
		return &expectedObjectError{a}
	}
	if x.Start != nil {
		return mapIterator(env.applyPipe(x.Start, unitIterator(a)), func(s interface{}) interface{} {
			if start, ok := toInt(s); ok {
				if x.End != nil {
					return mapIterator(env.applyPipe(x.End, unitIterator(a)), func(e interface{}) interface{} {
						if end, ok := toInt(e); ok {
							return applyArrayIndetInternal(&start, &end, nil, a)
						}
						return e
					})
				}
				if x.IsSlice {
					return applyArrayIndetInternal(&start, nil, nil, a)
				}
				return applyArrayIndetInternal(nil, nil, &start, a)
			}
			return s
		})
	}
	if x.End != nil {
		return mapIterator(env.applyPipe(x.End, unitIterator(a)), func(e interface{}) interface{} {
			if end, ok := toInt(e); ok {
				return applyArrayIndetInternal(nil, &end, nil, a)
			}
			return e
		})
	}
	return a
}

func indexIsForObject(x *Index) bool {
	return (x.Name != "" || x.String != nil || x.Start != nil) && !x.IsSlice && x.End == nil
}

func toInt(x interface{}) (int, bool) {
	switch x := x.(type) {
	case int:
		return x, true
	case float64:
		return int(x), true
	default:
		return 0, false
	}
}

func applyArrayIndetInternal(start, end, index *int, a []interface{}) interface{} {
	l := len(a)
	toIndex := func(i int) int {
		switch {
		case i < -l:
			return -2
		case i < 0:
			return l + i
		case i < l:
			return i
		default:
			return -1
		}
	}
	if index != nil {
		i := toIndex(*index)
		if i < 0 {
			return nil
		}
		return a[i]
	}
	if end != nil {
		i := toIndex(*end)
		if i == -1 {
			i = len(a)
		} else if i == -2 {
			i = 0
		}
		a = a[:i]
	}
	if start != nil {
		i := toIndex(*start)
		if i == -1 || len(a) < i {
			i = len(a)
		} else if i == -2 {
			i = 0
		}
		a = a[i:]
	}
	return a
}

func (env *env) applyFunc(f *Func, c <-chan interface{}) <-chan interface{} {
	if v, ok := env.lookupValue(f.Name); ok {
		return unitIterator(v)
	}
	if p := env.lookupVariable(f.Name); p != nil {
		return env.applyPipe(p, c)
	}
	if fn, ok := internalFuncs[f.Name]; ok {
		return mapIterator(c, fn(env, f))
	}
	fds := env.lookupFuncDef(f.Name)
	if fds == nil {
		return unitIterator(&funcNotFoundError{f})
	}
	fd, ok := fds[len(f.Args)]
	if !ok {
		return unitIterator(&funcNotFoundError{f})
	}
	subEnv := newEnv(env)
	var cc func() <-chan interface{}
	var d <-chan interface{}
	for i, arg := range fd.Args {
		if arg[0] == '$' {
			if cc == nil {
				cc = reuseIterator(c)
				d = unitIterator(map[string]interface{}{})
			}
			d = objectIterator(d,
				unitIterator(arg),
				env.applyPipe(f.Args[i], cc()))
		} else {
			subEnv.variables.Store(arg, f.Args[i])
		}
	}
	if d == nil {
		return subEnv.applyQuery(fd.Body, c)
	}
	return mapIterator(d, func(v interface{}) interface{} {
		m := v.(map[string]interface{})
		e := newEnv(env)
		subEnv.variables.Range(func(k, v interface{}) bool {
			e.variables.Store(k, v)
			return false
		})
		for k, v := range m {
			e.values.Store(k, v)
		}
		return e.applyQuery(fd.Body, cc())
	})
}

func (env *env) applyObject(x *Object, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		d := unitIterator(map[string]interface{}{})
		for _, kv := range x.KeyVals {
			if kv.KeyOnly != nil {
				if (*kv.KeyOnly)[0] == '$' {
					if vv, ok := env.lookupValue(*kv.KeyOnly); ok {
						d = objectIterator(d,
							unitIterator((*kv.KeyOnly)[1:]),
							unitIterator(vv))
					} else {
						return &variableNotFoundError{*kv.KeyOnly}
					}
				} else {
					d = objectIterator(d,
						unitIterator(*kv.KeyOnly),
						env.applyIndex(&Index{Name: *kv.KeyOnly}, unitIterator(v)))
				}
			} else if kv.KeyOnlyString != nil {
				d = objectKeyIterator(d,
					env.applyString(*kv.KeyOnlyString, unitIterator(v)),
					unitIterator(v))
			} else if kv.Pipe != nil {
				d = objectIterator(d,
					env.applyPipe(kv.Pipe, unitIterator(v)),
					env.applyExpr(kv.Val, unitIterator(v)))
			} else if kv.KeyString != nil {
				d = objectIterator(d,
					env.applyString(*kv.KeyString, unitIterator(v)),
					env.applyExpr(kv.Val, unitIterator(v)))
			} else {
				d = objectIterator(d,
					unitIterator(kv.Key),
					env.applyExpr(kv.Val, unitIterator(v)))
			}
		}
		return d
	})
}

func (env *env) applyArray(x *Array, c <-chan interface{}) <-chan interface{} {
	if x.Pipe == nil {
		return unitIterator([]interface{}{})
	}
	c = env.applyPipe(x.Pipe, c)
	a := []interface{}{}
	for v := range c {
		if err, ok := v.(error); ok {
			return unitIterator(err)
		}
		a = append(a, v)
	}
	return unitIterator(a)
}

func (env *env) applyString(x string, c <-chan interface{}) <-chan interface{} {
	if len(x) < 2 || x[0] != '"' || x[len(x)-1] != '"' {
		return unitIterator(&stringLiteralError{x})
	}
	orig := x
	// ref: strconv.Unquote
	x = x[1 : len(x)-1]
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(x)/2)
	var cc func() <-chan interface{}
	var xs []<-chan interface{}
	for len(x) > 0 {
		r, multibyte, ss, err := strconv.UnquoteChar(x, '"')
		if err != nil {
			if !strings.HasPrefix(x, "\\(") {
				return unitIterator(err)
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
				return unitIterator(&stringLiteralError{orig})
			}
			q, err := Parse(x[:i-1])
			if err != nil {
				return unitIterator(err)
			}
			x = x[i:]
			if len(buf) > 0 {
				xs = append(xs, unitIterator(string(buf)))
				buf = buf[:0]
			}
			if cc == nil {
				cc = reuseIterator(c)
			}
			xs = append(xs, env.applyQuery(q, cc()))
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
		return unitIterator(string(buf))
	}
	if len(buf) > 0 {
		xs = append(xs, unitIterator(string(buf)))
	}
	return stringIterator(xs)
}

func (env *env) applyUnary(op Operator, x *Term, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIterator(env.applyTerm(x, unitIterator(v)), func(v interface{}) interface{} {
			switch op {
			case OpAdd:
				switch v := v.(type) {
				case int:
					return v
				case float64:
					return v
				default:
					return &unaryTypeError{"plus", v}
				}
			case OpSub:
				switch v := v.(type) {
				case int:
					return -v
				case float64:
					return -v
				default:
					return &unaryTypeError{"negate", v}
				}
			default:
				panic(op)
			}
		})
	})
}

func (env *env) applySuffix(s *Suffix, c <-chan interface{}) <-chan interface{} {
	return mapIteratorWithError(c, func(v interface{}) interface{} {
		if s.Optional {
			switch v.(type) {
			case error:
				return struct{}{}
			default:
				return v
			}
		}
		if _, ok := v.(error); ok {
			return v
		}
		if x := s.Index; x != nil {
			return env.applyIndex(x, unitIterator(v))
		}
		if x := s.SuffixIndex; x != nil {
			return env.applyIndex(&Index{Start: x.Start, IsSlice: x.IsSlice, End: x.End}, unitIterator(v))
		}
		if s.Iter {
			return env.applyIterator(unitIterator(v))
		}
		panic("unreachable suffix")
	})
}

func (env *env) applyIterator(c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		if a, ok := v.([]interface{}); ok {
			d := make(chan interface{}, 1)
			go func() {
				defer close(d)
				for _, v := range a {
					d <- v
				}
			}()
			return (<-chan interface{})(d)
		} else if o, ok := v.(map[string]interface{}); ok {
			d := make(chan interface{}, 1)
			go func() {
				defer close(d)
				for _, v := range o {
					d <- v
				}
			}()
			return (<-chan interface{})(d)
		} else {
			return &iteratorError{v}
		}
	})
}

func (env *env) applyIf(x *If, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(x.Cond, unitIterator(v)), func(w interface{}) interface{} {
			if valueToBool(w) {
				return env.applyPipe(x.Then, unitIterator(v))
			}
			if len(x.Elif) > 0 {
				return env.applyIf(&If{x.Elif[0].Cond, x.Elif[0].Then, x.Elif[1:], x.Else}, unitIterator(v))
			}
			if x.Else != nil {
				return env.applyPipe(x.Else, unitIterator(v))
			}
			return v
		})
	})
}

func valueToBool(v interface{}) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	default:
		return true
	}
}

func (env *env) applyTry(x *Try, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIteratorWithError(env.applyPipe(x.Body, unitIterator(v)), func(w interface{}) interface{} {
			if err, ok := w.(error); ok {
				if x.Catch != nil {
					return env.applyPipe(x.Catch, unitIterator(err.Error()))
				}
				return struct{}{}
			}
			return w
		})
	})
}
