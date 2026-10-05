package gojq

import (
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ToInt returns the int value of v, and whether v is a number.
//
// This method is used by built-in operators and functions to convert numbers
// to integers, and accepts number types (int, float64, *big.Int, and
// json.Number); it returns false for other types. This method truncates
// floating-point numbers toward zero, and clamps numbers out of the int range
// (and infinities) to math.MinInt or math.MaxInt. NaN is converted to
// math.MinInt.
func ToInt(v any) (int, bool) {
	switch v := v.(type) {
	case int:
		return v, true
	case float64:
		return floatToInt(v), true
	case *big.Int:
		if v.IsInt64() {
			if i := v.Int64(); math.MinInt <= i && i <= math.MaxInt {
				return int(i), true
			}
		}
		if v.Sign() > 0 {
			return math.MaxInt, true
		}
		return math.MinInt, true
	case json.Number:
		return ToInt(parseNumber(v))
	default:
		return 0, false
	}
}

// ToFloat64 returns the float64 value of v, and whether v is a number.
//
// This method is used by built-in operators and functions to convert numbers
// to floating-point numbers, and accepts number types (int, float64, *big.Int,
// and json.Number); it returns false for other types. Note that large integers
// may lose precision, and numbers out of the float64 range are converted to
// infinities.
func ToFloat64(v any) (float64, bool) {
	switch v := v.(type) {
	case int:
		return float64(v), true
	case float64:
		return v, true
	case *big.Int:
		return bigToFloat(v), true
	case json.Number:
		return ToFloat64(parseNumber(v))
	default:
		return 0.0, false
	}
}

func floatToInt(x float64) int {
	if math.MinInt <= x && x < math.MaxInt {
		return int(x)
	}
	if x > 0 {
		return math.MaxInt
	}
	return math.MinInt
}

func bigToFloat(x *big.Int) float64 {
	if x.IsInt64() {
		return float64(x.Int64())
	}
	if f, err := strconv.ParseFloat(x.String(), 64); err == nil {
		return f
	}
	return math.Inf(x.Sign())
}

func parseNumber(v json.Number) any {
	if i, err := v.Int64(); err == nil && math.MinInt <= i && i <= math.MaxInt {
		return int(i)
	}
	if strings.ContainsAny(v.String(), ".eE") {
		if f, err := v.Float64(); err == nil {
			return f
		}
	}
	if bi, ok := new(big.Int).SetString(v.String(), 10); ok {
		return bi
	}
	if strings.HasPrefix(v.String(), "-") {
		return math.Inf(-1)
	}
	return math.Inf(1)
}
