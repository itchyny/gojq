package gojq_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/itchyny/gojq"
)

func TestPreview(t *testing.T) {
	testCases := []struct {
		value    any
		expected string
	}{
		{
			nil,
			"null",
		},
		{
			false,
			"false",
		},
		{
			true,
			"true",
		},
		{
			0,
			"0",
		},
		{
			3.14,
			"3.14",
		},
		{
			math.NaN(),
			"null",
		},
		{
			math.Inf(1),
			"1.7976931348623157e+308",
		},
		{
			math.Inf(-1),
			"-1.7976931348623157e+308",
		},
		{
			big.NewInt(9223372036854775807),
			"9223372036854775807",
		},
		{
			new(big.Int).SetBytes([]byte("\x0c\x9f\x2c\x9c\xd0\x46\x74\xed\xea\x3f\xff\xff\xff")),
			"999999999999999999999999999999",
		},
		{
			new(big.Int).SetBytes([]byte("\x0c\x9f\x2c\x9c\xd0\x46\x74\xed\xea\x40\x00\x00\x00")),
			"10000000000000000000000000 ...",
		},
		{
			"0 1 2 3 4 5 6 7 8 9 10 11 12",
			`"0 1 2 3 4 5 6 7 8 9 10 11 12"`,
		},
		{
			"0 1 2 3 4 5 6 7 8 9 10 11 12 13",
			`"0 1 2 3 4 5 6 7 8 9 10 1 ..."`,
		},
		{
			"０１２３４５６７８９",
			`"０１２３４５６７ ..."`,
		},
		{
			[]any{},
			"[]",
		},
		{
			[]any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12},
			"[0,1,2,3,4,5,6,7,8,9,10,11,12]",
		},
		{
			[]any{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13},
			"[0,1,2,3,4,5,6,7,8,9,10,1 ...]",
		},
		{
			[]any{[]any{[]any{[]any{[]any{[]any{[]any{[]any{nil, nil, nil}}}}}}}},
			"[[[[[[[[null,null,null]]]]]]]]",
		},
		{
			[]any{[]any{[]any{[]any{[]any{[]any{[]any{[]any{nil, nil, nil, nil}}}}}}}},
			"[[[[[[[[null,null,null,nu ...]",
		},
		{
			map[string]any{},
			"{}",
		},
		{
			map[string]any{"0": map[string]any{"1": map[string]any{"2": map[string]any{"3": []any{nil}}}}},
			`{"0":{"1":{"2":{"3":[null]}}}}`,
		},
		{
			map[string]any{"0": map[string]any{"1": map[string]any{"2": map[string]any{"3": map[string]any{"4": map[string]any{}}}}}},
			`{"0":{"1":{"2":{"3":{"4": ...}`,
		},
	}
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.value), func(t *testing.T) {
			got := gojq.Preview(tc.value)
			if got != tc.expected {
				t.Errorf("Preview(%v): got %s, expected %s", tc.value, got, tc.expected)
			}
		})
	}
}
