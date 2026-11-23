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
	if cmp, ok := cmpIntFloat(l, r); ok {
		return cmp
	}
	return binopTypeSwitch(l, r,
		cmp.Compare,
		cmpFloat,
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

func lt(l, r float64) bool {
	return l < r || math.IsNaN(l)
}

func cmpFloat(l, r float64) int {
	switch {
	case lt(l, r):
		return -1
	case l == r:
		return 0
	default:
		return 1
	}
}

// cmpIntFloat orders an integer against the exact value of a float64.
// The boolean is false when the pair is not a mixed integer and float.
//
// float64(n) and bigToFloat round integers past 2^53 and overflow integers
// past the float64 range to infinity. Equality is then non-transitive, so
// unique drops 2^53+1 or keeps it depending on input order, and a finite
// 2^1100 compares equal to +Inf.
func cmpIntFloat(l, r any) (int, bool) {
	switch l := l.(type) {
	case int:
		f, ok := r.(float64)
		if !ok {
			return 0, false
		}
		return cmpInt64Float(int64(l), f), true
	case *big.Int:
		f, ok := r.(float64)
		if !ok {
			return 0, false
		}
		return cmpBigFloat(l, f), true
	case float64:
		switch r := r.(type) {
		case int:
			return -cmpInt64Float(int64(r), l), true
		case *big.Int:
			return -cmpBigFloat(r, l), true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func cmpInt64Float(n int64, f float64) int {
	// Every integer in [-2^53, 2^53] is exactly representable as float64.
	if n >= -(1<<53) && n <= 1<<53 {
		return cmpFloat(float64(n), f)
	}
	return cmpBigFloat(big.NewInt(n), f)
}

func cmpBigFloat(n *big.Int, f float64) int {
	if n.IsInt64() {
		if i := n.Int64(); i >= -(1<<53) && i <= 1<<53 {
			return cmpFloat(float64(i), f)
		}
	}
	switch {
	case math.IsNaN(f):
		return 1
	case math.IsInf(f, 1):
		return -1
	case math.IsInf(f, -1):
		return 1
	case f == 0:
		return n.Sign()
	}
	// |n| >= 2^1024 is larger than every finite float64. Infinities are
	// handled above: a positive finite integer is still less than +Inf.
	if n.BitLen() > 1024 {
		return n.Sign()
	}
	bits := math.Float64bits(f)
	exp := int((bits >> 52) & 0x7ff)
	frac := bits & ((1 << 52) - 1)
	mant := frac
	if exp == 0 {
		// Subnormal: ±frac * 2^-1074.
		exp = -1074
	} else {
		mant |= 1 << 52
		// Normal: ±(2^52+frac) * 2^(exp-1023-52).
		exp = exp - 1023 - 52
	}
	m := new(big.Int).SetUint64(mant)
	if bits>>63 != 0 {
		m.Neg(m)
	}
	if exp > 0 {
		m.Lsh(m, uint(exp))
		return n.Cmp(m)
	}
	if exp < 0 {
		shifted := new(big.Int).Lsh(new(big.Int).Set(n), uint(-exp))
		return shifted.Cmp(m)
	}
	return n.Cmp(m)
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
