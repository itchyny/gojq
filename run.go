package gojq

// Run gojq.
func Run(q *Query, v interface{}) (interface{}, error) {
	v, err := applyPipe(q.Pipe, v)
	if err != nil {
		if err, ok := err.(*unexpectedQueryError); ok {
			err.q = q
			return nil, err
		}
		return nil, err
	}
	return v, nil
}

func applyPipe(pipe *Pipe, v interface{}) (interface{}, error) {
	var err error
	for _, c := range pipe.Commas {
		v, err = applyComma(c, v)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

func applyComma(c *Comma, v interface{}) (interface{}, error) {
	if w, ok := v.(chan interface{}); ok {
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			for e := range w {
				if _, ok := e.(error); ok {
					d <- e
					continue
				}
				x, err := applyComma(c, e)
				if err != nil {
					d <- err
					return
				}
				if y, ok := x.(chan interface{}); ok {
					for e := range y {
						d <- e
					}
					continue
				}
				d <- x
			}
		}()
		return d, nil
	}
	if len(c.Terms) == 1 {
		return applyTerm(c.Terms[0], v)
	}
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for _, t := range c.Terms {
			v, err := applyTerm(t, v)
			if err != nil {
				d <- err
				return
			}
			d <- v
		}
	}()
	return d, nil
}

func applyTerm(t *Term, v interface{}) (w interface{}, err error) {
	defer func() {
		for _, s := range t.SuffixList {
			w, err = applySuffix(s, w, err)
		}
	}()
	if x := t.ObjectIndex; x != nil {
		return applyObjectIndex(x, v)
	}
	if x := t.ArrayIndex; x != nil {
		return applyArrayIndex(x, v)
	}
	if t.Identity != nil {
		return v, nil
	}
	if t.Recurse != nil {
		return applyRecurse(v), nil
	}
	if x := t.Expression; x != nil {
		return applyExpression(x, v)
	}
	return nil, &unexpectedQueryError{}
}

func applyObjectIndex(x *ObjectIndex, v interface{}) (interface{}, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return nil, &expectedObjectError{v}
	}
	return m[x.Name], nil
}

func applyArrayIndex(x *ArrayIndex, v interface{}) (interface{}, error) {
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

func applyRecurse(v interface{}) chan interface{} {
	c := make(chan interface{}, 1)
	if a, ok := v.([]interface{}); ok {
		go func() {
			defer close(c)
			c <- v
			for _, d := range a {
				for e := range applyRecurse(d) {
					c <- e
				}
			}
		}()
	} else if o, ok := v.(map[string]interface{}); ok {
		go func() {
			defer close(c)
			c <- v
			for _, d := range o {
				for e := range applyRecurse(d) {
					c <- e
				}
			}
		}()
	} else if w, ok := v.(chan interface{}); ok {
		go func() {
			defer close(c)
			for d := range w {
				for e := range applyRecurse(d) {
					c <- e
				}
			}
		}()
	} else {
		go func() {
			defer close(c)
			c <- v
		}()
	}
	return c
}

func applyExpression(x *Expression, v interface{}) (interface{}, error) {
	if x.Null != nil {
		return nil, nil
	}
	if x.True != nil {
		return true, nil
	}
	if x.False != nil {
		return false, nil
	}
	if x.Object != nil {
		return applyObject(x.Object, v)
	}
	if x.Array != nil {
		return applyArray(x.Array, v)
	}
	return nil, &unexpectedQueryError{}
}

func applyObject(x *Object, v interface{}) (interface{}, error) {
	w := make(map[string]interface{})
	var iterators []iterator
	for _, kv := range x.KeyVals {
		key := kv.Key
		if kv.Pipe != nil {
			k, err := applyPipe(kv.Pipe, v)
			if err != nil {
				return nil, err
			}
			if l, ok := k.(string); ok {
				key = l
			} else {
				return nil, &objectKeyNotStringError{k}
			}
		}
		u, err := applyTerm(kv.Val, v)
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

func applyArray(x *Array, v interface{}) (interface{}, error) {
	if x.Pipe == nil {
		return []interface{}{}, nil
	}
	var err error
	v, err = applyPipe(x.Pipe, v)
	if err != nil {
		return nil, err
	}
	if w, ok := v.(chan interface{}); ok {
		v := []interface{}{}
		for e := range w {
			if err, ok := e.(error); ok {
				return nil, err
			}
			v = append(v, e)
		}
		return v, nil
	}
	return []interface{}{v}, nil
}

func applySuffix(s *Suffix, v interface{}, err error) (interface{}, error) {
	if v == struct{}{} {
		return v, nil
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
		return applyObjectIndex(x, v)
	}
	if x := s.ArrayIndex; x != nil {
		return applyArrayIndex(x, v)
	}
	if x := s.Array; x != nil {
		if x.Pipe == nil {
			return applyIterator(v)
		}
		return applyArray(x, v)
	}
	return nil, &unexpectedQueryError{}
}

func applyIterator(v interface{}) (chan interface{}, error) {
	c := make(chan interface{}, 1)
	if a, ok := v.([]interface{}); ok {
		go func() {
			defer close(c)
			for _, e := range a {
				c <- e
			}
		}()
	} else if o, ok := v.(map[string]interface{}); ok {
		go func() {
			defer close(c)
			for _, e := range o {
				c <- e
			}
		}()
	} else if w, ok := v.(chan interface{}); ok {
		go func() {
			defer close(c)
			for e := range w {
				if _, ok := e.(error); ok {
					c <- e
					continue
				}
				u, err := applyIterator(e)
				if err != nil {
					c <- err
					return
				}
				for x := range u {
					c <- x
				}
			}
		}()
	} else {
		close(c)
		return nil, &iteratorError{v}
	}
	return c, nil
}
