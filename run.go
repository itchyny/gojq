package gojq

// Run gojq.
func Run(q *Query, v interface{}) (interface{}, error) {
	if q.Term.Identity != nil {
		return v, nil
	}
	return nil, &unexpectedQueryError{q}
}
