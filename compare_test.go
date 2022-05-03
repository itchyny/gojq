package gojq_test

import (
	"fmt"
	"math"
	"math/big"
	"testing"

	"github.com/itchyny/gojq"
)

func TestCompare(t *testing.T) {
	testCases := []struct {
		l, r     interface{}
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
		{"", []interface{}{}, -1},
		{[]interface{}{}, "", 1},
		{[]interface{}{}, []interface{}{}, 0},
		{[]interface{}{}, []interface{}{nil}, -1},
		{[]interface{}{nil}, []interface{}{}, 1},
		{[]interface{}{nil}, []interface{}{nil}, 0},
		{[]interface{}{0, 1, 2}, []interface{}{0, 1, nil}, 1},
		{[]interface{}{0, 1, 2}, []interface{}{0, 1, 2, nil}, -1},
		{[]interface{}{0, 1, 2, false, nil}, []interface{}{0, 1, 2, nil, false}, 1},
		{[]interface{}{}, map[string]interface{}{}, -1},
		{map[string]interface{}{}, []interface{}{}, 1},
		{map[string]interface{}{}, map[string]interface{}{}, 0},
		{map[string]interface{}{"a": nil}, map[string]interface{}{"a": nil}, 0},
		{map[string]interface{}{"a": nil}, map[string]interface{}{"a": nil, "b": nil}, -1},
		{map[string]interface{}{"a": nil, "b": nil}, map[string]interface{}{"a": nil, "c": nil}, -1},
		{map[string]interface{}{"a": 0, "b": 0, "c": 0}, map[string]interface{}{"a": 0, "b": 0, "c": 0}, 0},
		{map[string]interface{}{"a": 0, "b": 0, "d": 0}, map[string]interface{}{"a": 0, "b": 1, "c": 0}, 1},
		{map[string]interface{}{"a": 0, "b": 1, "c": 2}, map[string]interface{}{"a": 0, "b": 2, "c": 1}, -1},
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
