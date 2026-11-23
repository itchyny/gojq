package gojq_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"testing"

	"github.com/itchyny/gojq"
)

func TestCompare(t *testing.T) {
	testCases := []struct {
		l, r     any
		expected int
	}{
		{nil, nil, 0},
		{nil, false, -1},
		{false, nil, 1},
		{false, false, 0},
		{false, true, -1},
		{true, false, 1},
		{true, true, 0},
		{true, 0, -1},
		{0, true, 1},
		{0, 0, 0},
		{0, 1, -1},
		{1, 0, 1},
		{1, 1, 0},
		{0, math.NaN(), 1},
		{math.NaN(), 0, -1},
		{math.NaN(), math.NaN(), -1},
		{1, 1.00, 0},
		{1.00, 1, 0},
		{1.00, 1.01, -1},
		{1.01, 1.00, 1},
		{1.01, 1.01, 0},
		{1, big.NewInt(0), 1},
		{big.NewInt(0), 1, -1},
		{0, big.NewInt(0), 0},
		{1, "", -1},
		{"", 1, 1},
		{"", "", 0},
		{"", "abc", -1},
		{"abc", "", 1},
		{"abc", "abc", 0},
		{"", []any{}, -1},
		{[]any{}, "", 1},
		{[]any{}, []any{}, 0},
		{[]any{}, []any{nil}, -1},
		{[]any{nil}, []any{}, 1},
		{[]any{nil}, []any{nil}, 0},
		{[]any{0, 1, 2}, []any{0, 1, nil}, 1},
		{[]any{0, 1, 2}, []any{0, 1, 2, nil}, -1},
		{[]any{0, 1, 2, false, nil}, []any{0, 1, 2, nil, false}, 1},
		{[]any{}, map[string]any{}, -1},
		{map[string]any{}, []any{}, 1},
		{map[string]any{}, map[string]any{}, 0},
		{map[string]any{"a": nil}, map[string]any{"a": nil}, 0},
		{map[string]any{"a": nil}, map[string]any{"a": nil, "b": nil}, -1},
		{map[string]any{"a": nil, "b": nil}, map[string]any{"a": nil, "c": nil}, -1},
		{map[string]any{"a": 0, "b": 0, "c": 0}, map[string]any{"a": 0, "b": 0, "c": 0}, 0},
		{map[string]any{"a": 0, "b": 0, "d": 0}, map[string]any{"a": 0, "b": 1, "c": 0}, 1},
		{map[string]any{"a": 0, "b": 1, "c": 2}, map[string]any{"a": 0, "b": 2, "c": 1}, -1},
		{big.NewInt(1), 1.0, 0},
		{1.0, big.NewInt(1), 0},
		{big.NewInt(1), 1.5, -1},
		{1.5, big.NewInt(1), 1},
		{big.NewInt(-2), -1.5, -1},
		{-1.5, big.NewInt(-2), 1},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v,%v", tc.l, tc.r), func(t *testing.T) {
			got := gojq.Compare(tc.l, tc.r)
			if got != tc.expected {
				t.Errorf("Compare(%v, %v): got %d, expected %d", tc.l, tc.r, got, tc.expected)
			}
		})
	}
}

func TestCompareMixedIntFloat(t *testing.T) {
	two53 := new(big.Int).Lsh(big.NewInt(1), 53)
	two53p1 := new(big.Int).Add(new(big.Int).Set(two53), big.NewInt(1))
	two100 := new(big.Int).Lsh(big.NewInt(1), 100)
	two100p1 := new(big.Int).Add(new(big.Int).Set(two100), big.NewInt(1))
	two1100 := new(big.Int).Lsh(big.NewInt(1), 1100)
	neg1100 := new(big.Int).Neg(new(big.Int).Set(two1100))
	neg53p1 := new(big.Int).Neg(new(big.Int).Set(two53p1))
	minInt := new(big.Int).SetInt64(math.MinInt64)
	minIntPlus := new(big.Int).Add(new(big.Int).Set(minInt), big.NewInt(1))
	maxFloatInt, acc := new(big.Float).SetPrec(53).SetFloat64(math.MaxFloat64).Int(nil)
	if acc != big.Exact {
		t.Fatalf("MaxFloat64 is not an integer: %v", acc)
	}
	maxFloatPlus := new(big.Int).Add(new(big.Int).Set(maxFloatInt), big.NewInt(1))
	next53 := math.Nextafter(9007199254740992.0, math.Inf(1))

	testCases := []mixedCompareCase{
		{"2^53 < 2^53+1", two53, two53p1, -1},
		{"2^53+1 > 2^53.0", two53p1, 9007199254740992.0, 1},
		{"2^53 == 2^53.0", two53, 9007199254740992.0, 0},
		{"2^53.0 < 2^53+1", 9007199254740992.0, two53p1, -1},
		{"2^53.0 == 2^53", 9007199254740992.0, two53, 0},
		{"2^53+1 < next float", two53p1, next53, -1},
		{"-(2^53+1) < -2^53.0", neg53p1, -9007199254740992.0, -1},
		{"-2^53 == -2^53.0", new(big.Int).Neg(new(big.Int).Set(two53)), -9007199254740992.0, 0},
		{"2^100 == 2^100.0", two100, 0x1p100, 0},
		{"2^100+1 > 2^100.0", two100p1, 0x1p100, 1},
		{"2^100.0 < 2^100+1", 0x1p100, two100p1, -1},
		{"2^63 == 2^63.0", new(big.Int).Lsh(big.NewInt(1), 63), 0x1p63, 0},
		{"MaxInt64 < float64(MaxInt64)", big.NewInt(math.MaxInt64), float64(math.MaxInt64), -1},
		{"MinInt64 == float64(MinInt64)", minInt, float64(math.MinInt64), 0},
		{"MinInt64+1 > float64(MinInt64)", minIntPlus, float64(math.MinInt64), 1},
		{"2^1100 < +Inf", two1100, math.Inf(1), -1},
		{"+Inf > 2^1100", math.Inf(1), two1100, 1},
		{"2^1100 > -Inf", two1100, math.Inf(-1), 1},
		{"-Inf < 2^1100", math.Inf(-1), two1100, -1},
		{"-2^1100 > -Inf", neg1100, math.Inf(-1), 1},
		{"-Inf < -2^1100", math.Inf(-1), neg1100, -1},
		{"2^1100 > MaxFloat64", two1100, math.MaxFloat64, 1},
		{"MaxFloat64 < 2^1100", math.MaxFloat64, two1100, -1},
		{"-2^1100 < -MaxFloat64", neg1100, -math.MaxFloat64, -1},
		{"MaxFloat64 == its integer", maxFloatInt, math.MaxFloat64, 0},
		{"MaxFloat64+1 > MaxFloat64", maxFloatPlus, math.MaxFloat64, 1},
		{"2^1100 > NaN", two1100, math.NaN(), 1},
		{"NaN < 2^1100", math.NaN(), two1100, -1},
		{"json 2^53+1 > 2^53.0", json.Number("9007199254740993"), 9007199254740992.0, 1},
		{"json 2^53 == 2^53.0", json.Number("9007199254740992"), 9007199254740992.0, 0},
		{"2^53.0 == json 2^53", 9007199254740992.0, json.Number("9007199254740992"), 0},
		{"json 2^1100 < +Inf", json.Number(two1100.String()), math.Inf(1), -1},
		{"0 == -0.0", big.NewInt(0), math.Copysign(0, -1), 0},
		{"1 > smallest subnormal", big.NewInt(1), math.SmallestNonzeroFloat64, 1},
		{"0 < smallest subnormal", big.NewInt(0), math.SmallestNonzeroFloat64, -1},
	}
	if i53, err := strconv.ParseInt(two53.String(), 10, strconv.IntSize); err == nil {
		i53p1, err := strconv.ParseInt(two53p1.String(), 10, strconv.IntSize)
		if err != nil {
			t.Fatal(err)
		}
		testCases = append(testCases,
			mixedCompareCase{"int 2^53 < int 2^53+1", int(i53), int(i53p1), -1},
			mixedCompareCase{"int 2^53+1 > 2^53.0", int(i53p1), 9007199254740992.0, 1},
			mixedCompareCase{"int 2^53 == 2^53.0", int(i53), 9007199254740992.0, 0},
			mixedCompareCase{"2^53.0 == int 2^53", 9007199254740992.0, int(i53), 0},
			mixedCompareCase{"2^53.0 < int 2^53+1", 9007199254740992.0, int(i53p1), -1},
		)
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := gojq.Compare(tc.l, tc.r)
			if got != tc.expected {
				t.Errorf("Compare(%v, %v): got %d, expected %d", tc.l, tc.r, got, tc.expected)
			}
		})
	}
}

func TestCompareIntFloatMatchesBigFloat(t *testing.T) {
	ns := []*big.Int{
		big.NewInt(0), big.NewInt(1), big.NewInt(-1),
		big.NewInt(2), big.NewInt(-3),
	}
	for _, exp := range []uint{52, 53, 54, 63, 64, 100, 1022, 1023, 1024, 1100} {
		p := new(big.Int).Lsh(big.NewInt(1), exp)
		ns = append(ns,
			new(big.Int).Set(p),
			new(big.Int).Add(new(big.Int).Set(p), big.NewInt(1)),
			new(big.Int).Sub(new(big.Int).Set(p), big.NewInt(1)),
			new(big.Int).Neg(p),
		)
	}
	fs := []float64{
		0, math.Copysign(0, -1),
		0.5, -0.5, 1, -1, 1.5, -1.5, 0.1, -0.1,
		math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		math.MaxFloat64, -math.MaxFloat64,
		math.Inf(1), math.Inf(-1), math.NaN(),
		9007199254740992.0, math.Nextafter(9007199254740992.0, math.Inf(1)),
		math.Nextafter(9007199254740992.0, 0),
		math.Nextafter(math.MaxFloat64, 0), math.Nextafter(-math.MaxFloat64, 0),
		math.Ldexp(1.5, 100), math.Ldexp(-1.5, 70),
	}
	for _, exp := range []int{-1074, -20, -1, 10, 53, 100, 1023} {
		fs = append(fs, math.Ldexp(1, exp), math.Ldexp(-1, exp))
	}
	for _, n := range ns {
		for _, f := range fs {
			got := gojq.Compare(n, f)
			want := bigFloatOrd(n, f)
			if got != want {
				t.Fatalf("Compare(%v, %g) = %d, big.Float = %d", n, f, got, want)
			}
			if rev := gojq.Compare(f, n); rev != -got {
				t.Fatalf("Compare(%g, %v) = %d, expected %d", f, n, rev, -got)
			}
		}
	}
}

type mixedCompareCase struct {
	name     string
	l, r     any
	expected int
}

func bigFloatOrd(n *big.Int, f float64) int {
	switch {
	case math.IsNaN(f):
		return 1
	case math.IsInf(f, 1):
		return -1
	case math.IsInf(f, -1):
		return 1
	}
	prec := uint(n.BitLen()) + 128
	if prec < 128 {
		prec = 128
	}
	bf := new(big.Float).SetPrec(prec).SetFloat64(f)
	bn := new(big.Float).SetPrec(prec).SetInt(n)
	return bn.Cmp(bf)
}
