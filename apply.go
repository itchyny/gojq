package gojq

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
			for _, e := range o.Exprs {
				for v := range env.applyExpr(e, unitIterator(v)) {
					d <- v
				}
			}
		}()
		return (<-chan interface{})(d)
	})
}

func (env *env) applyExpr(e *Expr, c <-chan interface{}) <-chan interface{} {
	if e.Logic != nil {
		return env.applyLogic(e.Logic, c)
	}
	return env.applyIf(e.If, c)
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

func (env *env) applyTerm(t *Term, c <-chan interface{}) (d <-chan interface{}) {
	defer func() {
		for _, s := range t.SuffixList {
			d = env.applySuffix(s, d)
		}
	}()
	if x := t.ObjectIndex; x != nil {
		return env.applyObjectIndex(x, c)
	}
	if x := t.ArrayIndex; x != nil {
		return env.applyArrayIndex(x, c)
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
		return unitIterator(*t.String)
	}
	return env.applyPipe(t.Pipe, c)
}

func (env *env) applyObjectIndex(x *ObjectIndex, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		m, ok := v.(map[string]interface{})
		if !ok {
			return &expectedObjectError{v}
		}
		return m[x.Name]
	})
}

func (env *env) applyArrayIndex(x *ArrayIndex, c <-chan interface{}) <-chan interface{} {
	return mapIterator(c, func(v interface{}) interface{} {
		a, ok := v.([]interface{})
		if !ok {
			return &expectedArrayError{v}
		}
		if x.Index != nil {
			return mapIterator(env.applyPipe(x.Index, unitIterator(a)), func(s interface{}) interface{} {
				if index, ok := toInt(s); ok {
					if x.End != nil {
						return mapIterator(env.applyPipe(x.End, unitIterator(a)), func(e interface{}) interface{} {
							if end, ok := toInt(e); ok {
								return applyArrayIndetInternal(&index, &end, nil, a)
							}
							return e
						})
					}
					if x.IsSlice {
						return applyArrayIndetInternal(&index, nil, nil, a)
					}
					return applyArrayIndetInternal(nil, nil, &index, a)
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
	})
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
	if v, ok := env.lookupValues(f.Name); ok {
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
				d = objectIterator(d,
					unitIterator(*kv.KeyOnly),
					env.applyObjectIndex(&ObjectIndex{*kv.KeyOnly}, unitIterator(v)))
			} else if kv.Pipe != nil {
				d = objectIterator(d,
					env.applyPipe(kv.Pipe, unitIterator(v)),
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
		if x := s.ObjectIndex; x != nil {
			return env.applyObjectIndex(x, unitIterator(v))
		}
		if x := s.ArrayIndex; x != nil {
			return env.applyArrayIndex(x, unitIterator(v))
		}
		if s.Array.Pipe == nil {
			return env.applyIterator(unitIterator(v))
		}
		return env.applyArray(s.Array, unitIterator(v))
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
			if condToBool(w) {
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

func condToBool(v interface{}) bool {
	switch v := v.(type) {
	case nil:
		return false
	case bool:
		return v
	default:
		return true
	}
}
