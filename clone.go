package gojq

func clone(v interface{}) interface{} {
	switch v := v.(type) {
	case map[string]interface{}:
		u := make(map[string]interface{}, len(v))
		for k, v := range v {
			u[k] = clone(v)
		}
		return u
	case []interface{}:
		u := make([]interface{}, len(v))
		for i, v := range v {
			u[i] = clone(v)
		}
		return u
	default:
		return v
	}
}
