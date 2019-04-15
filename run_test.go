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
			err:   "expected an object but got: number (128)",
		},
		{
			name:     "object optional",
			query:    `.foo.bar.baz?`,
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
			err:   `expected an array but got: object ({"foo":128})`,
		},
		{
			name:     "array iterator",
			query:    `.[]`,
			input:    []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
			iterator: true,
		},
		{
			name:     "array iterator optional",
			query:    `.[]?`,
			input:    10,
			expected: struct{}{},
		},
		{
			name:     "object iterator",
			query:    `.[]`,
			input:    map[string]interface{}{"a": 10},
			expected: []interface{}{10},
			iterator: true,
		},
		{
			name:     "object iterator optional",
			query:    `.foo?`,
			input:    []interface{}{"a"},
			expected: struct{}{},
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
			name:     "object construction",
			query:    `{ foo: .foo.bar, "bar": .foo, "": false }`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{1, 2, 3}}},
			expected: map[string]interface{}{"foo": []interface{}{1, 2, 3}, "bar": map[string]interface{}{"bar": []interface{}{1, 2, 3}}, "": false},
		},
		{
			name:  "iterator in object",
			query: `{ foo: .foo[], bar: .bar[], baz: .baz }`,
			input: map[string]interface{}{"foo": []interface{}{1, 2}, "bar": []interface{}{"a", "b"}, "baz": 128},
			expected: []interface{}{
				map[string]interface{}{"foo": 1, "bar": "a", "baz": 128},
				map[string]interface{}{"foo": 1, "bar": "b", "baz": 128},
				map[string]interface{}{"foo": 2, "bar": "a", "baz": 128},
				map[string]interface{}{"foo": 2, "bar": "b", "baz": 128},
			},
			iterator: true,
		},
		{
			name:     "pipe in object key",
			query:    `{ (.foo|.bar): .foo.bar }`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			expected: map[string]interface{}{"baz": "baz"},
		},
		{
			name:  "number in object key",
			query: `{ (.foo): .foo }`,
			input: map[string]interface{}{"foo": 10},
			err:   "expected a string for object key but got: number (10)",
		},
		{
			name:  "null in object key",
			query: `{ (.foo): .foo }`,
			input: map[string]interface{}{},
			err:   "expected a string for object key but got: null",
		},
		{
			name:     "array construction",
			query:    `. | [false, .foo, .bar.baz] | .`,
			input:    map[string]interface{}{"foo": "hello", "bar": map[string]interface{}{"baz": 128}},
			expected: []interface{}{false, "hello", 128},
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
		{
			name:     "iterator in array",
			query:    `[ .foo | .["bar"][][] ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{[]interface{}{1}, []interface{}{2}, []interface{}{3}}}},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:  "error after iterator",
			query: `[ .[] | .foo ]`,
			input: []interface{}{[]interface{}{1, 2, 3}, 2, 3},
			err:   "expected an object but got: ",
		},
		{
			name:     "multiple iterators in array",
			query:    `[.[],.[],..]`,
			input:    []interface{}{1, 2, 3},
			expected: []interface{}{1, 2, 3, 1, 2, 3, []interface{}{1, 2, 3}, 1, 2, 3},
		},
		{
			name:  "recurse",
			query: `..`,
			input: map[string]interface{}{"x": []interface{}{map[string]interface{}{"y": 128}, 48}},
			expected: []interface{}{
				map[string]interface{}{"x": []interface{}{map[string]interface{}{"y": 128}, 48}},
				[]interface{}{map[string]interface{}{"y": 128}, 48},
				map[string]interface{}{"y": 128},
				128,
				48,
			},
			iterator: true,
		},
		{
			name:  "recurse after iterator",
			query: `.[] | ..`,
			input: map[string]interface{}{"x": []interface{}{map[string]interface{}{"y": 128}}},
			expected: []interface{}{
				[]interface{}{map[string]interface{}{"y": 128}},
				map[string]interface{}{"y": 128},
				128,
			},
			iterator: true,
		},
		{
			name:     "length function",
			query:    `.[] | length`,
			input:    []interface{}{42, -42, map[string]interface{}{"a": 1, "b": 2, "c": 3}, []interface{}{4, 5}, "Hello, world", "あいうえお", nil},
			expected: []interface{}{42, 42, 3, 2, 12, 5, 0},
			iterator: true,
		},
		{
			name:  "length function error",
			query: `length`,
			input: false,
			err:   "length cannot be applied to: boolean (false)",
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
