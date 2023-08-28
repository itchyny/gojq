package gojq_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/itchyny/gojq"
)

func TestTypeOf(t *testing.T) {
	testCases := []struct {
		value    any
		expected string
	}{
		{nil, "null"},
		{false, "boolean"},
		{true, "boolean"},
		{0, "number"},
		{3.14, "number"},
		{math.NaN(), "number"},
		{math.Inf(1), "number"},
		{math.Inf(-1), "number"},
		{big.NewInt(10), "number"},
		{"string", "string"},
		{[]any{}, "array"},
		{map[string]any{}, "object"},
		{1, "number"},
		{int8(1), "number"},
		{int16(1), "number"},
		{int32(1), "number"},
		{int64(1), "number"},
		{uint8(1), "number"},
		{uint16(1), "number"},
		{uint32(1), "number"},
		{uint64(1), "number"},
		{float32(1), "number"},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			got := gojq.TypeOf(tc.value)
			if got != tc.expected {
				t.Errorf("TypeOf(%v): got %s, expected %s", tc.value, got, tc.expected)
			}
		})
	}
	func() {
		v := map[int]string{1: "foo"}
		defer func() {
			if got, expected := recover(), "invalid type: map[int]string (map[1:foo])"; got != expected {
				t.Errorf("TypeOf(%v) should panic: got %v, expected %v", v, got, expected)
			}
		}()
		_ = gojq.TypeOf(v)
	}()
}
