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
			name:     "array",
			query:    `.`,
			input:    []interface{}{"foo", 128},
			expected: []interface{}{"foo", 128},
		},
		{
			name:     "object index",
			query:    `.foo`,
			input:    map[string]interface{}{"foo": 128},
			expected: 128,
		},
	}

	for _, tc := range testCases {
		parser := NewParser()
		t.Run(tc.name, func(t *testing.T) {
			query, err := parser.Parse(tc.query)
			require.NoError(t, err)
			got, err := Run(query, tc.input)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, got)
		})
	}
}
