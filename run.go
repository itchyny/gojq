package gojq

// Run gojq.
func Run(q *Query, v interface{}) (interface{}, error) {
	if q.Term.Identity != nil {
		return v, nil
	}
	if x := q.Term.ObjectIndex; x != nil {
		m, ok := v.(map[string]interface{})
		if !ok {
			return nil, &expectedObjectError{v}
		}
		return m[x.Name[1:]], nil
	}
	return nil, &unexpectedQueryError{q}
}
