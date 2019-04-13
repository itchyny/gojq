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
				x, err := applyComma(c, e)
				if err != nil {
					panic(err) // todo
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
				panic(err) // todo
			}
			d <- v
		}
	}()
	return d, nil
}

func applyTerm(t *Term, v interface{}) (interface{}, error) {
	if t.Identity != nil {
		return v, nil
	}
	if x := t.ObjectIndex; x != nil {
		return applyObjectIndex(x, v)
	}
	if x := t.ArrayIndex; x != nil {
		return applyArrayIndex(x, v)
	}
	if x := t.Iterator; x != nil {
		return applyIterator(x, v)
	}
	if x := t.Expression; x != nil {
		return applyExpression(x, v)
	}
	return nil, &unexpectedQueryError{}
}

func applyObjectIndex(x *ObjectIndex, v interface{}) (interface{}, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		if x.Optional {
			return struct{}{}, nil
		}
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

func applyIterator(x *Iterator, v interface{}) (interface{}, error) {
	c := make(chan interface{}, 1)
	if x.Name != "" {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, &expectedObjectError{v}
		}
		v = m[x.Name]
	}
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
	} else {
		close(c)
		return nil, &iteratorError{v}
	}
	return c, nil
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
	if x.Array != nil {
		return applyArray(x.Array, v)
	}
	return nil, &unexpectedQueryError{}
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
			v = append(v, e)
		}
		return v, nil
	}
	return []interface{}{v}, nil
}
