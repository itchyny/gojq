package gojq

import (
	"cmp"
	"encoding/json"
	"math"
	"math/big"
	"strings"
)

// Compare l and r, and returns jq-flavored comparison value.
// The result will be 0 if l == r, -1 if l < r, and +1 if l > r.
// This comparison is used by built-in operators and functions.
func Compare(l, r any) int {
	if n, ok := l.(json.Number); ok {
		l = parseNumber(n)
	}
	if n, ok := r.(json.Number); ok {
		r = parseNumber(n)
	}
	switch l := l.(type) {
	case int:
		if r, ok := r.(float64); ok {
			return compareIntFloat(l, r)
		}
	case float64:
		switch r := r.(type) {
		case int:
			return -compareIntFloat(r, l)
		case *big.Int:
			return -compareBigIntFloat(r, l)
		}
	case *big.Int:
		if r, ok := r.(float64); ok {
			return compareBigIntFloat(l, r)
		}
	}
	return binopTypeSwitch(l, r,
		cmp.Compare,
		compareFloats,
		(*big.Int).Cmp,
		strings.Compare,
		func(l, r []any) int {
			for i := range min(len(l), len(r)) {
				if cmp := Compare(l[i], r[i]); cmp != 0 {
					return cmp
				}
			}
			return cmp.Compare(len(l), len(r))
		},
		func(l, r map[string]any) int {
			lk, rk := funcKeys(l), funcKeys(r)
			if cmp := Compare(lk, rk); cmp != 0 {
				return cmp
			}
			for _, k := range lk.([]any) {
				if cmp := Compare(l[k.(string)], r[k.(string)]); cmp != 0 {
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

func compareFloats(l, r float64) int {
	switch {
	case lt(l, r):
		return -1
	case l == r:
		return 0
	default:
		return 1
	}
}

func compareIntFloat(l int, r float64) int {
	if l := int64(l); -1<<53 <= l && l <= 1<<53 || math.IsNaN(r) {
		return compareFloats(float64(l), r)
	}
	return new(big.Float).SetInt64(int64(l)).Cmp(big.NewFloat(r))
}

func compareBigIntFloat(l *big.Int, r float64) int {
	if math.IsNaN(r) {
		return 1
	}
	return new(big.Float).SetInt(l).Cmp(big.NewFloat(r))
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
