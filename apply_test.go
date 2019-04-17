package gojq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	testCases := []struct {
		name     string
		src      string
		input    interface{}
		expected interface{}
		iterator bool
		err      string
	}{
		{
			name:     "number",
			src:      `.`,
			input:    128,
			expected: 128,
		},
		{
			name:     "string",
			src:      `.`,
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "object",
			src:      `.`,
			input:    map[string]interface{}{"foo": 128},
			expected: map[string]interface{}{"foo": 128},
		},
		{
			name:     "object index",
			src:      `.foo`,
			input:    map[string]interface{}{"foo": 128},
			expected: 128,
		},
		{
			name:     "object member",
			src:      `.["foo"]`,
			input:    map[string]interface{}{"foo": 128},
			expected: 128,
		},
		{
			name:  "expected object",
			src:   `.foo|.bar`,
			input: map[string]interface{}{"foo": 128},
			err:   "expected an object but got: number (128)",
		},
		{
			name:     "object optional",
			src:      `.foo.bar.baz?`,
			input:    map[string]interface{}{"foo": 128},
			expected: struct{}{},
		},
		{
			name:     "array index",
			src:      `.[2]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: 48,
		},
		{
			name:     "array index out of bound",
			src:      `. [ 4 ]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: nil,
		},
		{
			name:     "array slice start",
			src:      `.[2:]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{48, 64},
		},
		{
			name:     "array slice end",
			src:      `.[:2]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{16, 32},
		},
		{
			name:     "array slice start end",
			src:      `.[1:3]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{32, 48},
		},
		{
			name:     "array slice all",
			src:      `.[:]`,
			input:    []interface{}{16, 32, 48, 64},
			expected: []interface{}{16, 32, 48, 64},
		},
		{
			name:  "expected array",
			src:   `.[0]`,
			input: map[string]interface{}{"foo": 128},
			err:   `expected an array but got: object ({"foo":128})`,
		},
		{
			name:     "array iterator",
			src:      `.[]`,
			input:    []interface{}{"a", "b", "c"},
			expected: []interface{}{"a", "b", "c"},
			iterator: true,
		},
		{
			name:     "array iterator optional",
			src:      `.[]?`,
			input:    10,
			expected: struct{}{},
		},
		{
			name:     "object iterator",
			src:      `.[]`,
			input:    map[string]interface{}{"a": 10},
			expected: []interface{}{10},
			iterator: true,
		},
		{
			name:     "object iterator optional",
			src:      `.foo?`,
			input:    []interface{}{"a"},
			expected: struct{}{},
		},
		{
			name: "pipe",
			src:  `.foo | . | .baz | .[1]`,
			input: map[string]interface{}{
				"foo": map[string]interface{}{
					"baz": []interface{}{"Hello", "world"},
				},
			},
			expected: "world",
		},
		{
			name:     "null value",
			src:      `.[] | null`,
			input:    []interface{}{"a", 10, []interface{}{}},
			expected: []interface{}{nil, nil, nil},
			iterator: true,
		},
		{
			name:     "boolean values",
			src:      `.[] | true,false`,
			input:    []interface{}{"a", 10},
			expected: []interface{}{true, false, true, false},
			iterator: true,
		},
		{
			name:     "empty array",
			src:      `[]`,
			input:    []interface{}{1, 2, 3},
			expected: []interface{}{},
		},
		{
			name:     "object construction",
			src:      `{ foo: .foo.bar, "bar": .foo, "": false }`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{1, 2, 3}}},
			expected: map[string]interface{}{"foo": []interface{}{1, 2, 3}, "bar": map[string]interface{}{"bar": []interface{}{1, 2, 3}}, "": false},
		},
		{
			name:  "iterator in object",
			src:   `{ foo: .foo[], bar: .bar[], baz: .baz }`,
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
			src:      `{ (.foo|.bar): .foo.bar }`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": "baz"}},
			expected: map[string]interface{}{"baz": "baz"},
		},
		{
			name:  "number in object key",
			src:   `{ (.foo): .foo }`,
			input: map[string]interface{}{"foo": 10},
			err:   "expected a string for object key but got: number (10)",
		},
		{
			name:  "null in object key",
			src:   `{ (.foo): .foo }`,
			input: map[string]interface{}{},
			err:   "expected a string for object key but got: null",
		},
		{
			name:     "array construction",
			src:      `. | [false, .foo, .bar.baz] | .`,
			input:    map[string]interface{}{"foo": "hello", "bar": map[string]interface{}{"baz": 128}},
			expected: []interface{}{false, "hello", 128},
		},
		{
			name:     "pipe in array",
			src:      `[ .foo | .bar ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": 128}},
			expected: []interface{}{128},
		},
		{
			name:     "iterator in array",
			src:      `[ .[] | .foo ]`,
			input:    []interface{}{map[string]interface{}{"foo": "hello"}, map[string]interface{}{"foo": 128}},
			expected: []interface{}{"hello", 128},
		},
		{
			name:     "iterator in array",
			src:      `[ .foo | .bar[] ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{1, 2, 3}}},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:     "iterator in array",
			src:      `[ .foo | .["bar"][][] ]`,
			input:    map[string]interface{}{"foo": map[string]interface{}{"bar": []interface{}{[]interface{}{1}, []interface{}{2}, []interface{}{3}}}},
			expected: []interface{}{1, 2, 3},
		},
		{
			name:  "error after iterator",
			src:   `[ .[] | .foo ]`,
			input: []interface{}{[]interface{}{1, 2, 3}, 2, 3},
			err:   "expected an object but got: ",
		},
		{
			name:     "multiple iterators in array",
			src:      `[.[],.[],..]`,
			input:    []interface{}{1, 2, 3},
			expected: []interface{}{1, 2, 3, 1, 2, 3, []interface{}{1, 2, 3}, 1, 2, 3},
		},
		{
			name:  "recurse",
			src:   `..`,
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
			src:   `.[] | ..`,
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
			src:      `.[] | length`,
			input:    []interface{}{42, -42, map[string]interface{}{"a": 1, "b": 2, "c": 3}, []interface{}{4, 5}, "Hello, world", "あいうえお", nil},
			expected: []interface{}{42, 42, 3, 2, 12, 5, 0},
			iterator: true,
		},
		{
			name:     "utf8bytelength function",
			src:      `utf8bytelength`,
			input:    "あいうえお☆ミ",
			expected: 21,
		},
		{
			name:     "keys function",
			src:      `[.[] | keys]`,
			input:    []interface{}{map[string]interface{}{"c": 3, "b": 2, "a": 1}, []interface{}{3, 2, 1}},
			expected: []interface{}{[]interface{}{"a", "b", "c"}, []interface{}{0, 1, 2}},
		},
		{
			name:  "length function error",
			src:   `length`,
			input: false,
			err:   "length cannot be applied to: boolean (false)",
		},
		{
			name:     "function declaration",
			src:      `def f(g): g | g; f(..)`,
			input:    []interface{}{0, 1},
			expected: []interface{}{[]interface{}{0, 1}, 0, 1, 0, 1},
			iterator: true,
		},
		{
			name:  "argument count error",
			src:   `def f(g): g | g; f`,
			input: []interface{}{1},
			err:   "function not defined: f/0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := Parse(tc.src)
			assert.NoError(t, err)
			require.NoError(t, err)
			got, err := prog.Run(tc.input)
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
