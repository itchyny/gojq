package gojq

import (
	"math"
	"sort"
)

type function struct {
	minArgs, maxArgs int
	callback         func(interface{}) (interface{}, error)
}

var funcMap = map[string]function{
	"null":           {0, 0, funcNull},
	"true":           {0, 0, funcTrue},
	"false":          {0, 0, funcFalse},
	"length":         {0, 0, funcLength},
	"utf8bytelength": {0, 0, funcUtf8ByteLength},
	"keys":           {0, 0, funcKeys},
}

func (env *env) applyFunc(f *Func, v interface{}) (interface{}, error) {
	fn, ok := funcMap[f.Name]
	if !ok {
		return nil, &funcNotFoundError{f}
	}
	return fn.callback(v)
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
		if v > 0 {
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
