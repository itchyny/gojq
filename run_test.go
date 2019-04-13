package gojq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		input    interface{}
		expected interface{}
		iterator bool
		err      string
	}{
		{
			name:     "number",
			query:    `.`,
			input:    128,
			expected: 128,
		},
		{
			name:     "string",
			query:    `.`,
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "object",
			query:    `.`,
			input:    map[string]interface{}{"foo": 128},
			expected: map[string]interface{}{"foo": 128},
		},
		{
			name:     "object index",
			query:    `.foo`,
			input:    map[string]interface{}{"foo": 128},
			expected: 128,
		},
		{
			name:     "object member",
			query:    `.["foo"]`,
			input:    map[string]interface{}{"foo": 128},
			expected: 128,
		},
		{
			name:  "expected object",
			query: `.foo|.bar`,
			input: map[string]interface{}{"foo": 128},
			err:   "expected an object but got: int",
		},
		{
			name:     "object optional",
			query:    `.foo|.bar?`,
			input:    map[string]interface{}{"foo": 128},
			expected: struct{}{},
		},
		{
			name:     "array index",
			query:    `.[2]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: 48,
		},
		{
			name:     "array index out of bound",
			query:    `. [ 4 ]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: nil,
		},
		{
			name:     "array slice start",
			query:    `.[2:]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{48, 64},
		},
		{
			name:     "array slice end",
			query:    `.[:2]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{16, 32},
		},
		{
			name:     "array slice start end",
			query:    `.[1:3]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{32, 48},
		},
		{
			name:     "array slice all",
			query:    `.[:]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{16, 32, 48, 64},
		},
		{
			name:  "expected array",
			query: `.[0]`,
			input: map[string]interface{}{"foo": 128},
			err:   "expected an array but got: map",
		},
		{
			name:     "array iterator",
			query:    `.[]`,
			input:    []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
			iterator: true,
		},
		{
			name:     "object iterator",
			query:    `.[]`,
			input:    map[string]interface{}{"a": 10},
			expected: []interface{}{10},
			iterator: true,
		},
		{
			name:  "pipe",
			query: `.foo | . | .baz | .[1]`,
			input: map[string]interface{}{
				"foo": map[string]interface{}{
					"baz": []interface{}{"Hello", "world"},
				},
			},
			expected: "world",
		},
		{
			name:     "null value",
			query:    `.[] | null`,
			input:    []interface{}{"a", 10, []interface{}{}},
			expected: []interface{}{nil, nil, nil},
			iterator: true,
		},
		{
			name:     "boolean values",
			query:    `.[] | true,false`,
			input:    []interface{}{"a", 10},
			expected: []interface{}{true, false, true, false},
			iterator: true,
		},
		{
			name:     "empty array",
			query:    `[]`,
			input:    []interface{}{1, 2, 3},
			expected: []interface{}{},
		},
		{
			name:     "array",
			query:    `. | [false, .foo, .bar] | .`,
			input:    map[string]interface{}{"foo": "hello", "bar": map[string]interface{}{"baz": 128}},
			expected: []interface{}{false, "hello", map[string]interface{}{"baz": 128}},
		},
		{
			name:     "pipe in array",
			query:    `[ .foo | .bar ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": 128}},
			expected: []interface{}{128},
		},
		{
			name:     "iterator in array",
			query:    `[ .[] | .foo ]`,
			input:    []interface{}{map[string]interface{}{"foo": "hello"}, map[string]interface{}{"foo": 128}},
			expected: []interface{}{"hello", 128},
		},
		{
			name:     "iterator in array",
			query:    `[ .foo | .bar[] ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{1, 2, 3}}},
			expected: []interface{}{1, 2, 3},
		},
	}

	for _, tc := range testCases {
		parser := NewParser()
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			assert.NoError(t, err)
			require.NoError(t, err)
			got, err := Run(query, tc.input)
			if err == nil {
				require.NoError(t, err)
				if c, ok := got.(chan interface{}); ok {
					var got []interface{}
					for e := range c {
						got = append(got, e)
					}
					assert.Equal(t, tc.iterator, true)
					assert.Equal(t, tc.expected, got)
				} else {
					assert.Equal(t, tc.iterator, false)
					assert.Equal(t, tc.expected, got)
				}
			} else {
				assert.NotEqual(t, tc.err, "", err.Error())
				require.Contains(t, err.Error(), tc.err)
				assert.Equal(t, tc.expected, got)
			}
		})
	}
}
