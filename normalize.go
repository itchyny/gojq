package gojq

import (
	"encoding/json"
	"math"
	"math/big"
	"reflect"
	"strings"
)

func normalizeNumbers(v interface{}) interface{} {
	switch v := v.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i)
		}
		if strings.ContainsAny(string(v), ".eE") {
			if f, err := v.Float64(); err == nil {
				return f
			}
		}
		if bi, ok := new(big.Int).SetString(string(v), 10); ok {
			return bi
		}
		if strings.HasPrefix(string(v), "-") {
			return -math.MaxFloat64
		}
		return math.MaxFloat64
	case *big.Int:
		if v.IsInt64() {
			return int(v.Int64())
		}
		return v
	case int64:
		return int(v)
	case int32:
		return int(v)
	case map[string]interface{}:
		u := make(map[string]interface{}, len(v))
		for k, v := range v {
			u[k] = normalizeNumbers(v)
		}
		return u
	case []interface{}:
		u := make([]interface{}, len(v))
		for i, v := range v {
			u[i] = normalizeNumbers(v)
		}
		return u
	default:
		return v
	}
}

func normalizeValues(v interface{}) interface{} {
	switch v := v.(type) {
	case float64:
		if math.IsNaN(v) {
			return nil
		} else if isinf(v) {
			return math.Copysign(math.MaxFloat64, v)
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

// It's ok to delete destructively because this function is used right after
// updatePaths, where it shallow-copies maps or slices on updates.
func deleteEmpty(v interface{}) interface{} {
	switch v := v.(type) {
	case struct{}:
		return nil
	case map[string]interface{}:
		for k, w := range v {
			if w == struct{}{} {
				delete(v, k)
			} else {
				v[k] = deleteEmpty(w)
			}
		}
		return v
	case []interface{}:
		var j int
		for _, w := range v {
			if w != struct{}{} {
				v[j] = deleteEmpty(w)
				j++
			}
		}
		for i := j; i < len(v); i++ {
			v[i] = nil
		}
		return v[:j]
	default:
		return v
	}
}

func deepEqual(x, y interface{}) bool {
	return reflect.DeepEqual(normalizeNumbers(x), normalizeNumbers(y))
}
