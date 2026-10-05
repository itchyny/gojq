package gojq_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/itchyny/gojq"
)

func TestToInt(t *testing.T) {
	testCases := []struct {
		value    any
		expected int
		ok       bool
	}{
		{0, 0, true},
		{-42, -42, true},
		{3.7, 3, true},
		{-3.7, -3, true},
		{1e300, math.MaxInt, true},
		{-1e300, math.MinInt, true},
		{math.Inf(1), math.MaxInt, true},
		{math.Inf(-1), math.MinInt, true},
		{math.NaN(), math.MinInt, true},
		{big.NewInt(42), 42, true},
		{new(big.Int).Lsh(big.NewInt(1), 100), math.MaxInt, true},
		{new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 100)), math.MinInt, true},
		{json.Number("42"), 42, true},
		{json.Number("-3.7"), -3, true},
		{json.Number("100000000000000000000"), math.MaxInt, true},
		{json.Number("1e1000"), math.MaxInt, true},
		{json.Number("-1e1000"), math.MinInt, true},
		{nil, 0, false},
		{false, 0, false},
		{"42", 0, false},
		{[]any{}, 0, false},
		{map[string]any{}, 0, false},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			got, ok := gojq.ToInt(tc.value)
			if got != tc.expected || ok != tc.ok {
				t.Errorf("ToInt(%v): got (%d, %t), expected (%d, %t)",
					tc.value, got, ok, tc.expected, tc.ok)
			}
		})
	}
}

func TestToFloat64(t *testing.T) {
	testCases := []struct {
		value    any
		expected float64
		ok       bool
	}{
		{0, 0.0, true},
		{-42, -42.0, true},
		{3.14, 3.14, true},
		{math.Inf(1), math.Inf(1), true},
		{math.Inf(-1), math.Inf(-1), true},
		{math.NaN(), math.NaN(), true},
		{big.NewInt(42), 42.0, true},
		{new(big.Int).Lsh(big.NewInt(1), 100), 0x1p100, true},
		{new(big.Int).Lsh(big.NewInt(1), 1100), math.Inf(1), true},
		{new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 1100)), math.Inf(-1), true},
		{json.Number("42"), 42.0, true},
		{json.Number("-3.14"), -3.14, true},
		{json.Number("100000000000000000000"), 1e20, true},
		{json.Number("1e1000"), math.Inf(1), true},
		{json.Number("-1e1000"), math.Inf(-1), true},
		{nil, 0.0, false},
		{false, 0.0, false},
		{"3.14", 0.0, false},
		{[]any{}, 0.0, false},
		{map[string]any{}, 0.0, false},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			got, ok := gojq.ToFloat64(tc.value)
			if !(got == tc.expected || math.IsNaN(got) && math.IsNaN(tc.expected)) || ok != tc.ok {
				t.Errorf("ToFloat64(%v): got (%v, %t), expected (%v, %t)",
					tc.value, got, ok, tc.expected, tc.ok)
			}
		})
	}
}
