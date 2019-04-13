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
	for _, term := range pipe.Terms {
		v, err = applyTerm(term, v)
		if err != nil {
			return nil, err
		}
	}
	return v, nil
}

func applyTerm(term *Term, v interface{}) (interface{}, error) {
	if term.Identity != nil {
		return v, nil
	}
	if c, ok := v.(chan interface{}); ok {
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			for e := range c {
				x, err := applyTerm(term, e)
				if err != nil {
					panic(err) // todo
				}
				d <- x
			}
		}()
		return d, nil
	}
	if x := term.ObjectIndex; x != nil {
		return applyObjectIndex(x, v)
	}
	if x := term.ArrayIndex; x != nil {
		return applyArrayIndex(x, v)
	}
	if x := term.Iterator; x != nil {
		return applyIterator(v)
	}
	if x := term.Expression; x != nil {
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

func applyIterator(v interface{}) (interface{}, error) {
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
	return nil, &unexpectedQueryError{}
}
