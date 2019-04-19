package gojq

func (env *env) applyQuery(query *Query, v chan interface{}) chan interface{} {
	for _, fd := range query.FuncDefs {
		env.addFuncDef(fd)
	}
	return env.applyPipe(query.Pipe, v)
}

func (env *env) applyPipe(p *Pipe, v chan interface{}) chan interface{} {
	for _, c := range p.Commas {
		v = env.applyComma(c, v)
	}
	return v
}

func (env *env) applyComma(c *Comma, v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			for _, e := range c.Exprs {
				for e := range env.applyExpr(e, unitIterator(v)) {
					d <- e
				}
			}
		}()
		return d
	})
}

func (env *env) applyExpr(e *Expr, v chan interface{}) chan interface{} {
	if e.Term != nil {
		return env.applyTerm(e.Term, v)
	}
	return env.applyIf(e.If, v)
}

func (env *env) applyTerm(t *Term, v chan interface{}) (w chan interface{}) {
	defer func() {
		for _, s := range t.SuffixList {
			w = env.applySuffix(s, w)
		}
	}()
	if x := t.ObjectIndex; x != nil {
		return env.applyObjectIndex(x, v)
	}
	if x := t.ArrayIndex; x != nil {
		return env.applyArrayIndex(x, v)
	}
	if t.Identity != nil {
		return v
	}
	if t.Recurse != nil {
		return env.applyFunc(&Func{Name: "recurse"}, v)
	}
	if t.Func != nil {
		return env.applyFunc(t.Func, v)
	}
	if t.Object != nil {
		return env.applyObject(t.Object, v)
	}
	if t.Array != nil {
		return env.applyArray(t.Array, v)
	}
	return env.applyPipe(t.Pipe, v)
}

func (env *env) applyObjectIndex(x *ObjectIndex, v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		m, ok := v.(map[string]interface{})
		if !ok {
			return &expectedObjectError{v}
		}
		return m[x.Name]
	})
}

func (env *env) applyArrayIndex(x *ArrayIndex, v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		a, ok := v.([]interface{})
		if !ok {
			return &expectedArrayError{v}
		}
		if index := x.Index; index != nil {
			if *index < 0 || len(a) <= *index {
				return nil
			}
			return a[*index]
		}
		if end := x.End; end != nil {
			a = a[:*end]
		}
		if start := x.Start; start != nil {
			a = a[*start:]
		}
		return a
	})
}

func (env *env) applyFunc(f *Func, v chan interface{}) chan interface{} {
	if p := env.lookupVariable(f.Name); p != nil {
		return env.applyPipe(p, v)
	}
	if fn, ok := internalFuncs[f.Name]; ok {
		return mapIterator(v, fn)
	}
	fds := env.lookupFuncDef(f.Name)
	if fds == nil {
		return unitIterator(&funcNotFoundError{f})
	}
	fd, ok := fds[len(f.Args)]
	if !ok {
		return unitIterator(&funcArgCountError{f})
	}
	subEnv := newEnv(env)
	for i, arg := range fd.Args {
		subEnv.variables[arg] = f.Args[i]
	}
	return subEnv.applyQuery(fd.Body, v)
}

func (env *env) applyObject(x *Object, v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		w := make(map[string]interface{})
		var iterators []iterator
		for _, kv := range x.KeyVals {
			key := kv.Key
			if kv.Pipe != nil {
				var k interface{}
				var cnt int
				for k = range env.applyPipe(kv.Pipe, unitIterator(v)) {
					cnt++
					if cnt > 1 {
						break
					}
				}
				if l, ok := k.(string); ok && cnt == 1 {
					key = l
				} else {
					return &objectKeyNotStringError{k}
				}
			}
			iterators = append(iterators, iterator{key, env.applyExpr(kv.Val, unitIterator(v))})
		}
		if len(iterators) > 0 {
			return foldIterators(w, iterators)
		}
		return w
	})
}

func (env *env) applyArray(x *Array, v chan interface{}) chan interface{} {
	if x.Pipe == nil {
		return unitIterator([]interface{}{})
	}
	v = env.applyPipe(x.Pipe, v)
	a := []interface{}{}
	for e := range v {
		if err, ok := e.(error); ok {
			return unitIterator(err)
		}
		a = append(a, e)
	}
	return unitIterator(a)
}

func (env *env) applySuffix(s *Suffix, v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		if s.Optional {
			switch v.(type) {
			case *expectedObjectError, *expectedArrayError, *iteratorError:
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

func (env *env) applyIterator(v chan interface{}) chan interface{} {
	return mapIterator(v, func(v interface{}) interface{} {
		if a, ok := v.([]interface{}); ok {
			c := make(chan interface{}, 1)
			go func() {
				defer close(c)
				for _, e := range a {
					c <- e
				}
			}()
			return c
		} else if o, ok := v.(map[string]interface{}); ok {
			c := make(chan interface{}, 1)
			go func() {
				defer close(c)
				for _, e := range o {
					c <- e
				}
			}()
			return c
		} else {
			return &iteratorError{v}
		}
	})
}

func (env *env) applyIf(x *If, v chan interface{}) chan interface{} {
	t := reuseIterator(v)
	return mapIterator(env.applyPipe(x.Cond, t()), func(w interface{}) interface{} {
		if _, ok := w.(error); ok {
			return w
		}
		if condToBool(w) {
			return env.applyPipe(x.Then, t())
		}
		if len(x.Elif) > 0 {
			return env.applyIf(&If{x.Elif[0].Cond, x.Elif[0].Then, x.Elif[1:], x.Else}, t())
		}
		return env.applyPipe(x.Else, t())
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
