package gojq

func (env *env) applyQuery(query *Query, v interface{}) (interface{}, error) {
	if v == struct{}{} {
		return v, nil
	}
	for _, fd := range query.FuncDefs {
		env.addFuncDef(fd)
	}
	var err error
	v, err = env.applyPipe(query.Pipe, v)
	if err != nil {
		if err, ok := err.(*unexpectedQueryError); ok {
			err.q = query
			return nil, err
		}
		return nil, err
	}
	return v, nil
}

func (env *env) applyPipe(pipe *Pipe, v interface{}) (interface{}, error) {
	var err error
	for _, c := range pipe.Commas {
		v, err = env.applyComma(c, v)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

func (env *env) applyComma(c *Comma, v interface{}) (interface{}, error) {
	if w, ok := v.(chan interface{}); ok {
		return mapIterator(w, func(v interface{}) (interface{}, error) {
			if err, ok := v.(error); ok {
				return nil, err
			}
			return env.applyComma(c, v)
		}), nil
	}
	if len(c.Terms) == 1 {
		return env.applyTerm(c.Terms[0], v)
	}
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for _, t := range c.Terms {
			v, err := env.applyTerm(t, v)
			if err != nil {
				d <- err
				return
			}
			if w, ok := v.(chan interface{}); ok {
				for e := range w {
					d <- e
				}
				continue
			}
			d <- v
		}
	}()
	return d, nil
}

func (env *env) applyTerm(t *Term, v interface{}) (w interface{}, err error) {
	defer func() {
		for _, s := range t.SuffixList {
			w, err = env.applySuffix(s, w, err)
		}
	}()
	if x := t.ObjectIndex; x != nil {
		return env.applyObjectIndex(x, v)
	}
	if x := t.ArrayIndex; x != nil {
		return env.applyArrayIndex(x, v)
	}
	if t.Identity != nil {
		return v, nil
	}
	if t.Recurse != nil {
		return env.applyFunc(&Func{Name: "recurse"}, v)
	}
	if x := t.Expression; x != nil {
		return env.applyExpression(x, v)
	}
	if x := t.Pipe; x != nil {
		return env.applyPipe(x, v)
	}
	return nil, &unexpectedQueryError{}
}

func (env *env) applyObjectIndex(x *ObjectIndex, v interface{}) (interface{}, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, &expectedObjectError{v}
	}
	return m[x.Name], nil
}

func (env *env) applyArrayIndex(x *ArrayIndex, v interface{}) (interface{}, error) {
	a, ok := v.([]interface{})
	if !ok {
		return nil, &expectedArrayError{v}
	}
	if index := x.Index; index != nil {
		if *index < 0 || len(a) <= *index {
			return nil, nil
		}
		return a[*index], nil
	}
	if end := x.End; end != nil {
		a = a[:*end]
	}
	if start := x.Start; start != nil {
		a = a[*start:]
	}
	return a, nil
}

func (env *env) applyExpression(x *Expression, v interface{}) (interface{}, error) {
	if x.Func != nil {
		return env.applyFunc(x.Func, v)
	}
	if x.Object != nil {
		return env.applyObject(x.Object, v)
	}
	if x.Array != nil {
		return env.applyArray(x.Array, v)
	}
	return nil, &unexpectedQueryError{}
}

func (env *env) applyFunc(f *Func, v interface{}) (interface{}, error) {
	if p := env.lookupVariable(f.Name); p != nil {
		return env.applyPipe(p, v)
	}
	if fn, ok := internalFuncs[f.Name]; ok {
		return fn(v)
	}
	fds := env.lookupFuncDef(f.Name)
	if fds == nil {
		return nil, &funcNotFoundError{f}
	}
	fd, ok := fds[len(f.Args)]
	if !ok {
		return nil, &funcArgCountError{f}
	}
	subEnv := newEnv(env)
	for i, arg := range fd.Args {
		subEnv.variables[arg] = f.Args[i]
	}
	return subEnv.applyQuery(fd.Body, v)
}

func (env *env) applyObject(x *Object, v interface{}) (interface{}, error) {
	w := make(map[string]interface{})
	var iterators []iterator
	for _, kv := range x.KeyVals {
		key := kv.Key
		if kv.Pipe != nil {
			k, err := env.applyPipe(kv.Pipe, v)
			if err != nil {
				return nil, err
			}
			if l, ok := k.(string); ok {
				key = l
			} else {
				return nil, &objectKeyNotStringError{k}
			}
		}
		u, err := env.applyTerm(kv.Val, v)
		if err != nil {
			return nil, err
		}
		if t, ok := u.(chan interface{}); ok {
			iterators = append(iterators, iterator{key, t})
			continue
		}
		w[key] = u
	}
	if len(iterators) > 0 {
		return foldIterators(w, iterators), nil
	}
	return w, nil
}

func (env *env) applyArray(x *Array, v interface{}) (interface{}, error) {
	if x.Pipe == nil {
		return []interface{}{}, nil
	}
	var err error
	v, err = env.applyPipe(x.Pipe, v)
	if err != nil {
		return nil, err
	}
	if w, ok := v.(chan interface{}); ok {
		v := []interface{}{}
		for e := range w {
			if err, ok := e.(error); ok {
				return nil, err
			}
			if e == struct{}{} {
				continue
			}
			v = append(v, e)
		}
		return v, nil
	}
	return []interface{}{v}, nil
}

func (env *env) applySuffix(s *Suffix, v interface{}, err error) (interface{}, error) {
	if v == struct{}{} {
		return v, nil
	}
	if w, ok := v.(chan interface{}); ok {
		return mapIterator(w, func(v interface{}) (interface{}, error) {
			if err, ok := v.(error); ok {
				return env.applySuffix(s, nil, err)
			}
			return env.applySuffix(s, v, nil)
		}), nil
	}
	if s.Optional {
		switch err.(type) {
		case *expectedObjectError:
			return struct{}{}, nil
		case *expectedArrayError:
			return struct{}{}, nil
		case *iteratorError:
			return struct{}{}, nil
		default:
			return v, err
		}
	}
	if err != nil {
		return nil, err
	}
	if x := s.ObjectIndex; x != nil {
		return env.applyObjectIndex(x, v)
	}
	if x := s.ArrayIndex; x != nil {
		return env.applyArrayIndex(x, v)
	}
	if x := s.Array; x != nil {
		if x.Pipe == nil {
			return env.applyIterator(v)
		}
		return env.applyArray(x, v)
	}
	return nil, &unexpectedQueryError{}
}

func (env *env) applyIterator(v interface{}) (chan interface{}, error) {
	if a, ok := v.([]interface{}); ok {
		c := make(chan interface{}, 1)
		go func() {
			defer close(c)
			for _, e := range a {
				c <- e
			}
		}()
		return c, nil
	} else if o, ok := v.(map[string]interface{}); ok {
		c := make(chan interface{}, 1)
		go func() {
			defer close(c)
			for _, e := range o {
				c <- e
			}
		}()
		return c, nil
	} else if w, ok := v.(chan interface{}); ok {
		return mapIterator(w, func(v interface{}) (interface{}, error) {
			if err, ok := v.(error); ok {
				return nil, err
			}
			return env.applyIterator(v)
		}), nil
	} else {
		c := make(chan interface{}, 1)
		close(c)
		return c, &iteratorError{v}
	}
}
