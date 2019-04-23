package gojq

import (
	"fmt"
	"math"
	"sort"
	"strings"
)

type function func(*env, *Func) func(interface{}) interface{}

var internalFuncs map[string]function

func init() {
	internalFuncs = map[string]function{
		"null":           noArgFunc(funcNull),
		"true":           noArgFunc(funcTrue),
		"false":          noArgFunc(funcFalse),
		"empty":          noArgFunc(funcEmpty),
		"length":         noArgFunc(funcLength),
		"utf8bytelength": noArgFunc(funcUtf8ByteLength),
		"keys":           noArgFunc(funcKeys),
		"has":            funcHas,
		"join":           funcJoin,
	}
}

func noArgFunc(fn func(interface{}) interface{}) function {
	return func(_ *env, f *Func) func(interface{}) interface{} {
		return func(v interface{}) interface{} {
			if len(f.Args) != 0 {
				return &funcNotFoundError{f}
			}
			return fn(v)
		}
	}
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

func funcHas(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		if len(f.Args) != 1 {
			return &funcNotFoundError{f}
		}
		return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(x interface{}) interface{} {
			switch v := v.(type) {
			case []interface{}:
				switch x := x.(type) {
				case int:
					return 0 <= x && x < len(v)
				case float64:
					return 0 <= int(x) && int(x) < len(v)
				default:
					return &funcTypeError{"has", v}
				}
			case map[string]interface{}:
				switch x := x.(type) {
				case string:
					_, ok := v[x]
					return ok
				default:
					return &funcTypeError{"has", v}
				}
			default:
				return &funcTypeError{"has", v}
			}
		})
	}
}

func funcJoin(env *env, f *Func) func(interface{}) interface{} {
	return func(v interface{}) interface{} {
		if len(f.Args) != 1 {
			return &funcNotFoundError{f}
		}
		return mapIterator(env.applyPipe(f.Args[0], unitIterator(v)), func(x interface{}) interface{} {
			switch v := v.(type) {
			case []interface{}:
				switch x := x.(type) {
				case string:
					var s strings.Builder
					for i, v := range v {
						if i > 0 {
							s.WriteString(x)
						}
						s.WriteString(fmt.Sprint(v))
					}
					return s.String()
				default:
					return &funcTypeError{"join", v}
				}
			default:
				return &funcTypeError{"join", v}
			}
		})
	}
}
