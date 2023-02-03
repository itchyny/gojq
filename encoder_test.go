package gojq_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/itchyny/gojq"
)

func TestMarshal(t *testing.T) {
	testCases := []struct {
		value    any
		expected string
	}{
		{
			value:    nil,
			expected: "null",
		},
		{
			value:    []any{false, true},
			expected: "[false,true]",
		},
		{
			value: []any{
				42, 3.14, 1e-6, 1e-7, -1e-9, 1e-10, math.NaN(), math.Inf(1), math.Inf(-1),
				new(big.Int).SetBytes([]byte("\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff")),
			},
			expected: "[42,3.14,0.000001,1e-7,-1e-9,1e-10,null,1.7976931348623157e+308,-1.7976931348623157e+308,340282366920938463463374607431768211455]",
		},
		{
			value:    []any{"", "abcde", "foo\x00\x1f\r\n\t\f\b<=>!\"#$%'& \\\x7fbar"},
			expected: `["","abcde","foo\u0000\u001f\r\n\t\f\b<=>!\"#$%'& \\\u007fbar"]`,
		},
		{
			value:    []any{1, []any{2, []any{3, []any{map[string]any{}}}}},
			expected: `[1,[2,[3,[{}]]]]`,
		},
		{
			value:    map[string]any{"x": []any{100}, "y": map[string]any{"z": 42}},
			expected: `{"x":[100],"y":{"z":42}}`,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			got, err := gojq.Marshal(tc.value)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.expected {
				t.Errorf("expected: %s, got: %s", tc.expected, string(got))
			}
		})
	}
}
