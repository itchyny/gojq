package gojq

import (
	"math"
	"sort"
)

type function func(interface{}) interface{}

var internalFuncs = map[string]function{
	"null":           funcNull,
	"true":           funcTrue,
	"false":          funcFalse,
	"empty":          funcEmpty,
	"length":         funcLength,
	"utf8bytelength": funcUtf8ByteLength,
	"keys":           funcKeys,
}

func funcNull(_ interface{}) interface{} {
	return nil
}

func funcTrue(_ interface{}) interface{} {
	return true
}

func funcFalse(_ interface{}) interface{} {
	return false
}

func funcEmpty(_ interface{}) interface{} {
	return struct{}{}
}

func funcLength(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return len(v)
	case map[string]interface{}:
		return len(v)
	case string:
		return len([]rune(v))
	case int:
		if v >= 0 {
			return v
		}
		return -v
	case float64:
		return math.Abs(v)
	case nil:
		return 0
	default:
		return &funcTypeError{"length", v}
	}
}

func funcUtf8ByteLength(v interface{}) interface{} {
	switch v := v.(type) {
	case string:
		return len([]byte(v))
	default:
		return &funcTypeError{"utf8bytelength", v}
	}
}

func funcKeys(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		w := make([]interface{}, len(v))
		for i := range v {
			w[i] = i
		}
		return w
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
		return u
	default:
		return &funcTypeError{"keys", v}
	}
}
