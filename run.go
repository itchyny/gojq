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
	if x := term.ObjectIndex; x != nil {
		m, ok := v.(map[string]interface{})
		if !ok {
			if x.Optional {
				return struct{}{}, nil
			}
			return nil, &expectedObjectError{v}
		}
		return m[x.Name], nil
	}
	if x := term.ArrayIndex; x != nil {
		a, ok := v.([]interface{})
		if !ok {
			return nil, &expectedArrayError{v}
		}
		if x.Index < 0 || len(a) <= x.Index {
			return nil, nil
		}
		return a[x.Index], nil
	}
	return nil, &unexpectedQueryError{}
}
