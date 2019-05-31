package gojq

import "math"

func normalizeValues(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		if math.IsNaN(v) {
			return nil
		} else if math.IsInf(v, 0) {
			if v > 0 {
				return math.MaxFloat64
			}
			return -math.MaxFloat64
		} else {
			return v
		}
	case map[string]interface{}:
		u := make(map[string]interface{}, len(v))
		for k, v := range v {
			u[k] = normalizeValues(v)
		}
		return u
	case []interface{}:
		u := make([]interface{}, len(v))
		for i, v := range v {
			u[i] = normalizeValues(v)
		}
		return u
	default:
		return v
	}
}
