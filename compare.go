package gojq

import (
	"cmp"
	"encoding/json"
	"math"
	"math/big"
)

// Compare l and r, and returns jq-flavored comparison value.
// The result will be 0 if l == r, -1 if l < r, and +1 if l > r.
// This comparison is used by built-in operators and functions.
func Compare(l, r any) int {
	return compareDepth(l, r, 0)
}

// compareDepth is Compare with a recursion-depth bound. Compare recurses on
// nested arrays and objects; a deeply nested value ( >= 16 bytes per level )
// would overflow the goroutine stack ( a fatal crash ) inside sort, unique, min,
// the comparison operators, etc. Past the depth a value already exceeds the
// limit, so panic; the interpreter's Next recovers it.
func compareDepth(l, r any, depth int) int {
	if MaxAlloc > 0 && (int64(depth)*16 > MaxAlloc || depth > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	return binopTypeSwitch(l, r,
		cmp.Compare,
		func(l, r float64) int {
			switch {
			case lt(l, r):
				return -1
			case l == r:
				return 0
			default:
				return 1
			}
		},
		(*big.Int).Cmp,
		cmp.Compare,
		func(l, r []any) int {
			for i := range min(len(l), len(r)) {
				if cmp := compareDepth(l[i], r[i], depth+1); cmp != 0 {
					return cmp
				}
			}
			return cmp.Compare(len(l), len(r))
		},
		func(l, r map[string]any) int {
			lk, rk := funcKeys(l), funcKeys(r)
			if cmp := compareDepth(lk, rk, depth+1); cmp != 0 {
				return cmp
			}
			for _, k := range lk.([]any) {
				if cmp := compareDepth(l[k.(string)], r[k.(string)], depth+1); cmp != 0 {
					return cmp
				}
			}
			return 0
		},
		func(l, r any) int {
			return cmp.Compare(typeIndex(l), typeIndex(r))
		},
	)
}

func lt(l, r float64) bool {
	return l < r || math.IsNaN(l)
}

func typeIndex(v any) int {
	switch v := v.(type) {
	default:
		return 0
	case bool:
		if !v {
			return 1
		}
		return 2
	case int, float64, *big.Int, json.Number:
		return 3
	case string:
		return 4
	case []any:
		return 5
	case map[string]any:
		return 6
	}
}
