package gojq

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

func (env *env) applyQuery(query *Query, c Iter) Iter {
	for _, fd := range query.FuncDefs {
		env.addFuncDef(fd)
	}
	return env.applyPipe(query.Pipe, c)
}

func (env *env) applyPipe(p *Pipe, c Iter) Iter {
	for _, o := range p.Commas {
		c = env.applyComma(o, c)
	}
	return c
}

func (env *env) applyComma(o *Comma, c Iter) Iter {
	return mapIterator(c, func(v interface{}) interface{} {
		if len(o.Alts) == 1 {
			return env.applyAlt(o.Alts[0], unitIterator(v))
		}
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			for _, e := range o.Alts {
				for v := range env.applyAlt(e, unitIterator(v)) {
					d <- v
				}
			}
		}()
		return (Iter)(d)
	})
}

func (env *env) applyAlt(e *Alt, c Iter) Iter {
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

func (env *env) applyExpr(e *Expr, c Iter) Iter {
	if e.Bind == nil {
		return env.applyExprInternal(e, c)
	}
	if e.Bind.Pattern.Name != "" && e.Bind.Pattern.Name[0] != '$' {
		return unitIterator(&bindVariableNameError{e.Bind.Pattern.Name})
	}
	cc := reuseIterator(c)
	return mapIterator(env.applyExprInternal(e, cc()), func(v interface{}) interface{} {
		subEnv := newEnv(env)
		if err := subEnv.applyPattern(e.Bind.Pattern, v); err != nil {
			return err
		}
		return subEnv.applyPipe(e.Bind.Body, cc())
	})
}

func (env *env) applyExprInternal(e *Expr, c Iter) Iter {
	if e.Logic != nil {
		return env.applyLogic(e.Logic, c)
	}
	if e.If != nil {
		return env.applyIf(e.If, c)
	}
	if e.Try != nil {
		return env.applyTry(e.Try, c)
	}
	if e.Reduce != nil {
		return env.applyReduce(e.Reduce, c)
	}
	if e.Foreach != nil {
		return env.applyForeach(e.Foreach, c)
	}
	if e.Label != nil {
		return env.applyLabel(e.Label, c)
	}
	panic("unreachable expr")
}

func (env *env) applyLogic(e *Logic, c Iter) Iter {
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

func (env *env) applyAndExpr(e *AndExpr, c Iter) Iter {
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

func (env *env) applyCompare(e *Compare, c Iter) Iter {
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

func (env *env) applyArith(e *Arith, c Iter) Iter {
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

func (env *env) applyFactor(e *Factor, c Iter) Iter {
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

func (env *env) applyTerm(t *Term, c Iter) (d Iter) {
	cc := func() Iter { return c }
	if t.Index != nil || t.SuffixList != nil {
		cc = reuseIterator(c)
	}
	defer func() {
		for _, s := range t.SuffixList {
			d = env.applySuffix(s, d, cc)
		}
	}()
	if x := t.Index; x != nil {
		return env.applyIndex(x, cc(), cc())
	}
	if t.Identity {
		return cc()
	}
	if t.Recurse {
		return env.applyFunc(&Func{Name: "recurse"}, cc())
	}
	if t.Func != nil {
		return env.applyFunc(t.Func, cc())
	}
	if t.Object != nil {
		return env.applyObject(t.Object, cc())
	}
	if t.Array != nil {
		return env.applyArray(t.Array, cc())
	}
	if t.Number != nil {
		return unitIterator(*t.Number)
	}
	if t.Unary != nil {
		return env.applyUnary(t.Unary.Op, t.Unary.Term, cc())
	}
	if t.String != "" {
		return env.applyString(t.String, cc())
	}
	if t.Null {
		return unitIterator(nil)
	}
	if t.True {
		return unitIterator(true)
	}
	if t.False {
		return unitIterator(false)
	}
	if t.Break != "" {
		return unitIterator(&breakError{t.Break})
	}
	return env.applyPipe(t.Pipe, cc())
}

func (env *env) applyIndex(x *Index, c Iter, d Iter) Iter {
	dd := reuseIterator(d)
	return mapIterator(c, func(v interface{}) interface{} {
		switch v := v.(type) {
		case nil:
			return nil
		case map[string]interface{}:
			return env.applyObjectIndex(x, v, dd())
		case []interface{}:
			return env.applyArrayIndex(x, v, dd())
		case string:
			switch v := env.applyArrayIndex(x, explode(v), dd()).(type) {
			case Iter:
				return mapIterator(v, func(v interface{}) interface{} {
					switch v := v.(type) {
					case []interface{}:
						return implode(v)
					case int:
						return implode([]interface{}{v})
					case nil:
						return ""
					default:
						panic(v)
					}
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

func (env *env) applyObjectIndex(x *Index, m map[string]interface{}, c Iter) interface{} {
	if !indexIsForObject(x) {
		return &expectedArrayError{m}
	}
	if x.Name != "" {
		return m[x.Name]
	}
	if x.String != "" {
		return mapIterator(env.applyString(x.String, c), func(s interface{}) interface{} {
			key, ok := s.(string)
			if !ok {
				return &objectKeyNotStringError{s}
			}
			return m[key]
		})
	}
	return mapIterator(env.applyPipe(x.Start, c), func(s interface{}) interface{} {
		key, ok := s.(string)
		if !ok {
			return &objectKeyNotStringError{s}
		}
		return m[key]
	})
}

func (env *env) applyArrayIndex(x *Index, a []interface{}, c Iter) interface{} {
	cc := func() Iter { return c }
	if x.Start != nil && x.End != nil {
		cc = reuseIterator(c)
	}
	if x.Name != "" {
		return &expectedObjectError{a}
	}
	if x.Start != nil {
		return mapIterator(env.applyPipe(x.Start, cc()), func(s interface{}) interface{} {
			if start, ok := toInt(s); ok {
				if x.End != nil {
					return mapIterator(env.applyPipe(x.End, cc()), func(e interface{}) interface{} {
						if end, ok := toInt(e); ok {
							return applyArrayIndetInternal(&start, &end, nil, a)
						}
						return &arrayIndexNotNumberError{e}
					})
				}
				if x.IsSlice {
					return applyArrayIndetInternal(&start, nil, nil, a)
				}
				return applyArrayIndetInternal(nil, nil, &start, a)
			} else if b, ok := s.([]interface{}); ok {
				var xs []interface{}
				if len(b) == 0 {
					return xs
				}
				for i := 0; i < len(a) && i < len(a)-len(b)+1; i++ {
					var neq bool
					for j, y := range b {
						if neq = compare(a[i+j], y) != 0; neq {
							break
						}
					}
					if !neq {
						xs = append(xs, i)
					}
				}
				return xs
			}
			return &arrayIndexNotNumberError{s}
		})
	}
	if x.End != nil {
		return mapIterator(env.applyPipe(x.End, cc()), func(e interface{}) interface{} {
			if end, ok := toInt(e); ok {
				return applyArrayIndetInternal(nil, &end, nil, a)
			}
			return &arrayIndexNotNumberError{e}
		})
	}
	return a
}

func indexIsForObject(x *Index) bool {
	return (x.Name != "" || x.String != "" || x.Start != nil) && !x.IsSlice && x.End == nil
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

func (env *env) applyFunc(f *Func, c Iter) Iter {
	if v, ok := env.lookupValue(f.Name); ok {
		return unitIterator(v)
	}
	if p := env.lookupVariable(f.Name); p != nil {
		return env.parent.applyPipe(p, c)
	}
	if f.Name[0] == '$' {
		return unitIterator(&variableNotFoundError{f.Name})
	}
	if fn, ok := internalFuncs[f.Name]; ok {
		return mapIterator(c, fn.callback(env, f))
	}
	fd := env.lookupFuncDef(f.Name, len(f.Args))
	if fd == nil {
		return unitIterator(&funcNotFoundError{f})
	}
	subEnv := newEnv(env)
	var cc func() Iter
	var d Iter
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

func (env *env) applyObject(x *Object, c Iter) Iter {
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
						env.applyIndex(&Index{Name: *kv.KeyOnly}, unitIterator(v), unitIterator(v)))
				}
			} else if kv.KeyOnlyString != "" {
				d = objectKeyIterator(d,
					env.applyString(kv.KeyOnlyString, unitIterator(v)),
					unitIterator(v))
			} else if kv.Pipe != nil {
				d = objectIterator(d,
					env.applyPipe(kv.Pipe, unitIterator(v)),
					env.applyExpr(kv.Val, unitIterator(v)))
			} else if kv.KeyString != "" {
				d = objectIterator(d,
					env.applyString(kv.KeyString, unitIterator(v)),
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

func (env *env) applyArray(x *Array, c Iter) Iter {
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

func (env *env) applyString(x string, c Iter) Iter {
	if len(x) < 2 || x[0] != '"' || x[len(x)-1] != '"' {
		return unitIterator(&stringLiteralError{x})
	}
	orig := x
	// ref: strconv.Unquote
	x = x[1 : len(x)-1]
	var runeTmp [utf8.UTFMax]byte
	buf := make([]byte, 0, 3*len(x)/2)
	var cc func() Iter
	var xs []Iter
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

func (env *env) applyUnary(op Operator, x *Term, c Iter) Iter {
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

func (env *env) applySuffix(s *Suffix, c Iter, cc func() Iter) Iter {
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
			return env.applyIndex(x, unitIterator(v), cc())
		}
		if x := s.SuffixIndex; x != nil {
			return env.applyIndex(&Index{Start: x.Start, IsSlice: x.IsSlice, End: x.End}, unitIterator(v), cc())
		}
		if s.Iter {
			return env.applyIterator(unitIterator(v))
		}
		panic("unreachable suffix")
	})
}

func (env *env) applyIterator(c Iter) Iter {
	return mapIterator(c, func(v interface{}) interface{} {
		if a, ok := v.([]interface{}); ok {
			d := make(chan interface{}, 1)
			go func() {
				defer close(d)
				for _, v := range a {
					d <- v
				}
			}()
			return (Iter)(d)
		} else if o, ok := v.(map[string]interface{}); ok {
			d := make(chan interface{}, 1)
			go func() {
				defer close(d)
				for _, v := range o {
					d <- v
				}
			}()
			return (Iter)(d)
		} else {
			return &iteratorError{v}
		}
	})
}

func (env *env) applyIf(x *If, c Iter) Iter {
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

func (env *env) applyTry(x *Try, c Iter) Iter {
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

func (env *env) applyReduce(x *Reduce, c Iter) Iter {
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(x.Start, unitIterator(v)), func(s interface{}) interface{} {
			subEnv := newEnv(env)
			return foldIterator(subEnv.applyTerm(x.Term, unitIterator(v)), s, func(v, w interface{}) interface{} {
				if err := subEnv.applyPattern(x.Pattern, w); err != nil {
					return err
				}
				return iteratorLast(subEnv.applyPipe(x.Update, unitIterator(v)))
			})
		})
	})
}

func (env *env) applyForeach(x *Foreach, c Iter) Iter {
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIterator(env.applyPipe(x.Start, unitIterator(v)), func(s interface{}) interface{} {
			subEnv := newEnv(env)
			return foreachIterator(subEnv.applyTerm(x.Term, unitIterator(v)), s, func(v, w interface{}) (interface{}, Iter) {
				if err := subEnv.applyPattern(x.Pattern, w); err != nil {
					return err, unitIterator(err)
				}
				u := reuseIterator(subEnv.applyPipe(x.Update, unitIterator(v)))
				if x.Extract == nil {
					return iteratorLast(u()), u()
				}
				return iteratorLast(u()), subEnv.applyPipe(x.Extract, u())
			})
		})
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
	} else if len(p.Object) > 0 {
		if v == nil {
			v = map[string]interface{}{}
		}
		m, ok := v.(map[string]interface{})
		if !ok {
			return &expectedObjectError{v}
		}
		for _, o := range p.Object {
			if o.KeyOnly != "" {
				key := o.KeyOnly
				if key[0] != '$' {
					return &bindVariableNameError{key}
				}
				env.values.Store(key, m[key[1:]])
				continue
			}
			key := o.Key
			if key != "" && key[0] == '$' {
				env.values.Store(key, m[key[1:]])
				key = key[1:]
			}
			if o.KeyString != "" {
				key = o.KeyString
			}
			if err := env.applyPattern(o.Val, m[key]); err != nil {
				return err
			}
		}
	}
	return nil
}

func (env *env) applyLabel(x *Label, c Iter) Iter {
	if x.Ident[0] != '$' {
		return unitIterator(&labelNameError{x.Ident})
	}
	return mapIterator(c, func(v interface{}) interface{} {
		return mapIteratorWithError(env.applyPipe(x.Body, unitIterator(v)), func(v interface{}) interface{} {
			if e, ok := v.(*breakError); ok && x.Ident == e.n {
				return struct{}{}
			}
			return v
		})
	})
}
