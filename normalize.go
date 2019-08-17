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

func deleteEmpty(v interface{}) interface{} {
	switch v := v.(type) {
	case struct{}:
		return nil
	case map[string]interface{}:
		u := make(map[string]interface{}, len(v))
		for k, v := range v {
			if v == struct{}{} {
				continue
			}
			u[k] = deleteEmpty(v)
		}
		return u
	case []interface{}:
		u := make([]interface{}, 0, len(v))
		for _, v := range v {
			if v == struct{}{} {
				continue
			}
			u = append(u, deleteEmpty(v))
		}
		return u
	default:
		return v
	}
}

func deepEqual(x, y interface{}) bool {
	return reflect.DeepEqual(normalizeNumbers(x), normalizeNumbers(y))
}
