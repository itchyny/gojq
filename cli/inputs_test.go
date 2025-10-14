package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestJSONInputIter(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input       string
		expected    []any
		expectedErr string
	}{
		{
			name:     "empty",
			input:    "",
			expected: []any{},
		},
		{
			name:     "scalars",
			input:    "null true false 1 2.3 \"hello\"",
			expected: []any{nil, true, false, json.Number("1"), json.Number("2.3"), "hello"},
		},
		{
			name:  "arrays",
			input: "[][[]][1.2][3,[4]]",
			expected: []any{
				[]any{},
				[]any{[]any{}},
				[]any{json.Number("1.2")},
				[]any{json.Number("3"), []any{json.Number("4")}},
			},
		},
		{
			name:  "objects",
			input: `{}{"a":1,"b":2}{"a":{"b":3,"c":4}}`,
			expected: []any{
				map[string]any{},
				map[string]any{"a": json.Number("1"), "b": json.Number("2")},
				map[string]any{"a": map[string]any{"b": json.Number("3"), "c": json.Number("4")}},
			},
		},
		{
			name:     "unexpected EOF error",
			input:    "0[1",
			expected: []any{json.Number("0")},
			expectedErr: `invalid json: test.json
    0[1
       ^  unexpected EOF`,
		},
		{
			name:     "array value error",
			input:    `0["a",]`,
			expected: []any{json.Number("0")},
			expectedErr: `invalid json: test.json
    0["a",]
          ^  invalid character ']' looking for beginning of value`,
		},
		{
			name:     "object key error",
			input:    "0\n{\n  0",
			expected: []any{json.Number("0")},
			expectedErr: `invalid json: test.json:3
    3 |   0
          ^  invalid character '0' looking for beginning of object key string`,
		},
		{
			name:     "object value error",
			input:    "0\n{\n  \"a\":\n}",
			expected: []any{json.Number("0")},
			expectedErr: `invalid json: test.json:4
    4 | }
        ^  invalid character '}' looking for beginning of value`,
		},
		{
			name:     "large input with unexpected EOF error",
			input:    "0[0," + strings.Repeat("\n", 40*1024) + "1\n",
			expected: []any{json.Number("0")},
			expectedErr: fmt.Sprintf(`invalid json: test.json:%[1]d
    %[1]d | 1
             ^  unexpected EOF`, 40*1024+1),
		},
		{
			name:     "large input with array value error",
			input:    "0[0," + strings.Repeat("\n", 40*1024) + "]",
			expected: []any{json.Number("0")},
			expectedErr: fmt.Sprintf(`invalid json: test.json:%[1]d
    %[1]d | ]
            ^  invalid character ']' looking for beginning of value`, 40*1024+1),
		},
		{
			name:     "large input with object key error",
			input:    `0{"a"` + strings.Repeat("\n", 40*1024) + ":0,1}",
			expected: []any{json.Number("0")},
			expectedErr: fmt.Sprintf(`invalid json: test.json:%[1]d
    %[1]d | :0,1}
               ^  invalid character '1' looking for beginning of object key string`, 40*1024+1),
		},
		{
			name:     "many input values with value error",
			input:    strings.Repeat("0\n", 40*1024) + ":\n",
			expected: slices.Repeat([]any{json.Number("0")}, 40*1024),
			expectedErr: fmt.Sprintf(`invalid json: test.json:%[1]d
    %[1]d | :
            ^  invalid character ':' looking for beginning of value`, 40*1024+1),
		},
	} {
		for _, r := range []io.Reader{strings.NewReader(tc.input), newStringReader(tc.input)} {
			t.Run(fmt.Sprintf("%s_%T", tc.name, r), func(t *testing.T) {
				iter := newJSONInputIter(r, "test.json")
				got, gotErr := []any{}, error(nil)
				for {
					v, ok := iter.Next()
					if !ok {
						break
					}
					if gotErr, ok = v.(error); ok {
						continue
					}
					got = append(got, v)
				}
				if !reflect.DeepEqual(got, tc.expected) {
					t.Errorf("newJSONInputIter(%T).Next():\n"+
						"     got: %#v\n"+
						"expected: %#v", r, got, tc.expected)
				}
				if (tc.expectedErr == "") != (gotErr == nil) ||
					gotErr != nil && gotErr.Error() != tc.expectedErr {
					t.Errorf("newJSONInputIter(%T).Next():\n"+
						"     got error: %v\n"+
						"expected error: %v", r, gotErr, tc.expectedErr)
				}
			})
		}
	}
}

func TestYAMLInputIter(t *testing.T) {
	for _, tc := range []struct {
		name        string
		input       string
		expected    []any
		expectedErr string
	}{
		{
			name:     "empty",
			input:    "",
			expected: []any{},
		},
		{
			name:     "scalars",
			input:    "null\n---\ntrue\n---\nfalse\n---\n1\n---\n2.3\n---\nhello",
			expected: []any{nil, true, false, json.Number("1"), json.Number("2.3"), "hello"},
		},
		{
			name:  "arrays",
			input: "[]\n---\n- []\n---\n- 1.2\n---\n- 3\n- - 4",
			expected: []any{
				[]any{},
				[]any{[]any{}},
				[]any{json.Number("1.2")},
				[]any{json.Number("3"), []any{json.Number("4")}},
			},
		},
		{
			name:  "objects",
			input: "{}\n---\na: 1\nb: 2\n---\na:\n  b: 3\n  c: 4",
			expected: []any{
				map[string]any{},
				map[string]any{"a": json.Number("1"), "b": json.Number("2")},
				map[string]any{"a": map[string]any{"b": json.Number("3"), "c": json.Number("4")}},
			},
		},
		{
			name:     "unexpected EOF error",
			input:    "0\n---\n[",
			expected: []any{json.Number("0")},
			expectedErr: `invalid yaml: test.yaml:3
    3 | [
         ^  did not find expected node content`,
		},
		{
			name:     "large input with unexpected EOF error",
			input:    strings.Repeat("0\n---\n", 20*1024) + "{",
			expected: slices.Repeat([]any{json.Number("0")}, 20*1024),
			expectedErr: fmt.Sprintf(`invalid yaml: test.yaml:%[1]d
    %[1]d | {
             ^  did not find expected node content`, 40*1024+1),
		},
	} {
		for _, r := range []io.Reader{strings.NewReader(tc.input), newStringReader(tc.input)} {
			t.Run(fmt.Sprintf("%s_%T", tc.name, r), func(t *testing.T) {
				iter := newYAMLInputIter(r, "test.yaml")
				got, gotErr := []any{}, error(nil)
				for {
					v, ok := iter.Next()
					if !ok {
						break
					}
					if gotErr, ok = v.(error); ok {
						continue
					}
					got = append(got, v)
				}
				if !reflect.DeepEqual(got, tc.expected) {
					t.Errorf("newYAMLInputIter(%T).Next():\n"+
						"     got: %#v\n"+
						"expected: %#v", r, got, tc.expected)
				}
				if (tc.expectedErr == "") != (gotErr == nil) ||
					gotErr != nil && gotErr.Error() != tc.expectedErr {
					t.Errorf("newYAMLInputIter(%T).Next():\n"+
						"     got error: %v\n"+
						"expected error: %v", r, gotErr, tc.expectedErr)
				}
			})
		}
	}
}
