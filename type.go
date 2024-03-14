package gojq

import (
	"fmt"
	"math/big"
	"reflect"
	"time"

	"github.com/modopayments/go-modo/v8"
	"github.com/modopayments/go-modo/v8/uuid"
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
	case uint, uint8, uint16, uint32, uint64, int, int8, int16, int32, int64, float32, float64, *big.Int:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	case uuid.UUID:
		return "uuid"
	case uuid.NullUUID:
		return "nullUUID"
	case time.Time, modo.Timestamp:
		return "time"
	default:
		t := reflect.TypeOf(v)
		switch t.Kind() {
		case reflect.Ptr:
			v := reflect.ValueOf(v)
			if v.IsNil() {
				return "null"
			}
			return "*" + TypeOf(v.Elem().Interface())
		case reflect.Struct:
			return "struct"
		case reflect.Slice: // this an interface{} that happens to mask a []any
			return "array"
		case reflect.Map:
			return "object"
		default:
			panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
		}
	}
}
