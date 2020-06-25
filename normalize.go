package gojq

import (
	"encoding/json"
	"math"
	"math/big"
	"strings"
)

func normalizeNumbers(v interface{}) interface{} {
	switch v := v.(type) {
	case json.Number:
		if i, err := v.Int64(); err == nil && minInt <= i && i <= maxInt {
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
			if i := v.Int64(); minInt <= i && i <= maxInt {
				return int(i)
			}
		}
		return v
	case int64:
		if v > int64(maxInt) {
			return new(big.Int).SetUint64(uint64(v))
		}
		return int(v)
	case int32:
		return int(v)
	case int16:
		return int(v)
	case int8:
		return int(v)
	case uint:
		if v > uint(maxInt) {
			return new(big.Int).SetUint64(uint64(v))
		}
		return int(v)
	case uint64:
		if v > uint64(maxInt) {
			return new(big.Int).SetUint64(v)
		}
		return int(v)
	case uint32:
		if v > uint32(maxHalfInt) {
			return new(big.Int).SetUint64(uint64(v))
		}
		return int(v)
	case uint16:
		return int(v)
	case uint8:
		return int(v)
	case float32:
		return float64(v)
	case map[string]interface{}:
		for k, x := range v {
			v[k] = normalizeNumbers(x)
		}
		return v
	case []interface{}:
		for i, x := range v {
			v[i] = normalizeNumbers(x)
		}
		return v
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
