package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCliRun(t *testing.T) {
	testCases := []struct {
		name     string
		args     []string
		input    string
		expected string
		err      string
	}{
		{
			name:     "empty",
			args:     []string{},
			input:    ``,
			expected: ``,
		},
		{
			name:  "number",
			args:  []string{"."},
			input: `128`,
			expected: `128
`,
		},
		{
			name:  "object",
			args:  []string{},
			input: `{"foo": 128}`,
			expected: `{
  "foo": 128
}
`,
		},
		{
			name:  "iterator",
			args:  []string{".foo | .[] | ."},
			input: `{"foo": [1,2,{"bar":[]},[3,4,5]]}`,
			expected: `1
2
{
  "bar": []
}
[
  3,
  4,
  5
]
`,
		},
		{
			name:  "pipe",
			args:  []string{".foo | .bar"},
			input: `{"foo": {"bar": {"baz": 128}}}`,
			expected: `{
  "baz": 128
}
`,
		},
		{
			name:     "object optional",
			args:     []string{".foo | .bar?"},
			input:    `{"foo": 128}`,
			expected: ``,
		},
		{
			name:  "iterator in object",
			args:  []string{"{ foo: .foo[], bar: .bar[] }"},
			input: `{"foo":[1,2],"bar":["a","b"]}`,
			expected: `{
  "bar": "a",
  "foo": 1
}
{
  "bar": "b",
  "foo": 1
}
{
  "bar": "a",
  "foo": 2
}
{
  "bar": "b",
  "foo": 2
}
`,
		},
		{
			name:  "array",
			args:  []string{"[.foo, .]"},
			input: `{"foo": {"bar": 128}}`,
			expected: `[
  {
    "bar": 128
  },
  {
    "foo": {
      "bar": 128
    }
  }
]
`,
		},
		{
			name:  "pipe in array",
			args:  []string{"[.foo|.bar]"},
			input: `{"foo": {"bar": 128}}`,
			expected: `[
  128
]
`,
		},
		{
			name:  "iterator in array",
			args:  []string{"[.foo|.bar[][]]"},
			input: `{"foo": {"bar": [[1],[2],[3]]}}`,
			expected: `[
  1,
  2,
  3
]
`,
		},
		{
			name:  "recurse",
			args:  []string{".."},
			input: `{"foo":{"bar":["a","b"]}}`,
			expected: `{
  "foo": {
    "bar": [
      "a",
      "b"
    ]
  }
}
{
  "bar": [
    "a",
    "b"
  ]
}
[
  "a",
  "b"
]
"a"
"b"
`,
		},
		{
			name:  "recurse and pipe",
			args:  []string{".. | .foo?"},
			input: `{"foo":{"bar":["a","b"]}}`,
			expected: `{
  "bar": [
    "a",
    "b"
  ]
}
null
`,
		},
		{
			name:  "length function",
			args:  []string{"[.[]|length]"},
			input: `[42, -42, {"a":1,"b":2}, [3,4,5], "Hello, world", "あいうえお", null]`,
			expected: `[
  42,
  42,
  2,
  3,
  12,
  5,
  0
]
`,
		},
		{
			name:  "function not defined",
			args:  []string{"abc"},
			input: `{}`,
			err: `function not defined: abc
`,
		},
		{
			name:  "invalid query",
			args:  []string{">abc"},
			input: `{}`,
			err: `invalid query: >abc
    >abc
    ^  invalid syntax
`,
		},
		{
			name:  "invalid query",
			args:  []string{".abc["},
			input: `{}`,
			err: `invalid query: .abc[
    .abc[
        ^  unexpected token "["
`,
		},
		{
			name:  "invalid query",
			args:  []string{"[ .[] | { id } ]"},
			input: `{}`,
			err: `invalid query: [ .[] | { id } ]
    [ .[] | { id } ]
          ^  unexpected "|" (expected "]")
`,
		},
		{
			name:  "invalid json: eof",
			args:  []string{},
			input: `{`,
			err: `invalid json: unexpected EOF
    {
     ^
`,
		},
		{
			name: "invalid json: invalid character",
			args: []string{},
			input: `{
  "あいうえお" 100
}`,
			err: `invalid json: invalid character '1' after object key
      "あいうえお" 100
                   ^
`,
		},
		{
			name: "invalid json: string literal",
			args: []string{},
			input: `{
  "いろは": "
}`,
			err: `invalid json: invalid character '\n' in string literal
      "いろは": "
                 ^
`,
		},
		{
			name:  "multiple json",
			args:  []string{},
			input: `{}[]{"foo":10}{"bar":[]}`,
			expected: `{}
[]
{
  "foo": 10
}
{
  "bar": []
}
`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			outStream := new(bytes.Buffer)
			errStream := new(bytes.Buffer)
			cli := cli{
				inStream:  strings.NewReader(tc.input),
				outStream: outStream,
				errStream: errStream,
			}
			code := cli.run(tc.args)
			if tc.err == "" {
				assert.Equal(t, exitCodeOK, code)
				assert.Equal(t, tc.expected, outStream.String())
			} else {
				assert.Equal(t, exitCodeErr, code)
				assert.Contains(t, errStream.String(), tc.err)
			}
		})
	}
}
