package gojq

import (
	"math"
	"sort"
)

type function func(interface{}) (interface{}, error)

var internalFuncs = map[string]function{
	"null":           funcNull,
	"true":           funcTrue,
	"false":          funcFalse,
	"length":         funcLength,
	"utf8bytelength": funcUtf8ByteLength,
	"keys":           funcKeys,
}

func funcNull(_ interface{}) (interface{}, error) {
	return nil, nil
}

func funcTrue(_ interface{}) (interface{}, error) {
	return true, nil
}

func funcFalse(_ interface{}) (interface{}, error) {
	return false, nil
}

func funcLength(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case []interface{}:
		return len(v), nil
	case map[string]interface{}:
		return len(v), nil
	case string:
		return len([]rune(v)), nil
	case int:
		if v >= 0 {
			return v, nil
		}
		return -v, nil
	case float64:
		return math.Abs(v), nil
	case nil:
		return 0, nil
	default:
		return nil, &funcTypeError{"length", v}
	}
}

func funcUtf8ByteLength(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case string:
		return len([]byte(v)), nil
	default:
		return nil, &funcTypeError{"utf8bytelength", v}
	}
}

func funcKeys(v interface{}) (interface{}, error) {
	switch v := v.(type) {
	case []interface{}:
		w := make([]interface{}, len(v))
		for i := range v {
			w[i] = i
		}
		return w, nil
	case map[string]interface{}:
		w := make([]string, len(v))
		var i int
		for k := range v {
			w[i] = k
			i++
		}
		sort.Strings(w)
		u := make([]interface{}, len(v))
		for i, x := range w {
			u[i] = x
		}
		return u, nil
	default:
		return nil, &funcTypeError{"keys", v}
	}
}
