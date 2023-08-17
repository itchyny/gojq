package gojq

import (
	"fmt"
	"math/big"
	"reflect"
)

// TypeOf returns the jq-flavored type name of v.
//
// This method is used by built-in type/0 function, and accepts only limited
// types (nil, bool, int, float64, *big.Int, string, []any, and map[string]any).
func TypeOf(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case int, float64, *big.Int:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case fmt.Stringer:
		return "stringer"
	default:
		t := reflect.TypeOf(v)
		switch t.Kind() {
		case reflect.Ptr:
			return TypeOf(reflect.ValueOf(t).Elem().Interface())
		case reflect.Struct:
			return "struct"
		case reflect.Slice:
			return "array"
		default:
			panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
		}
	}
}
