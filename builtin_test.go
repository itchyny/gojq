package gojq_test

import (
	"fmt"
	"testing"

	"github.com/itchyny/gojq"
)

func TestBase64url(t *testing.T) {
	testCases := []struct {
		query    string
		input    any
		expected []any
	}{
		{
			query:    "@base64url",
			input:    "hello",
			expected: []any{"aGVsbG8="},
		},
		{
			query:    "@base64url",
			input:    ">?>?",
			expected: []any{"Pj8-Pw=="},
		},
		{
			query:    "@base64url",
			input:    ">?>???",
			expected: []any{"Pj8-Pz8_"},
		},
		{
			query:    "@base64url",
			input:    map[string]any{"foo": "bar"},
			expected: []any{"eyJmb28iOiJiYXIifQ=="},
		},
		{
			query:    `@base64url "\(.),\(. + .)"`,
			input:    []any{1, 2},
			expected: []any{"WzEsMl0=,WzEsMiwxLDJd"},
		},
		{
			query:    `format("base64url")`,
			input:    ">?>?",
			expected: []any{"Pj8-Pw=="},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.query, func(t *testing.T) {
			query, err := gojq.Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			iter := query.Run(tc.input)
			var got []any
			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					t.Fatal(err)
				}
				got = append(got, v)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.expected) {
				t.Fatalf("expected: %v, got: %v", tc.expected, got)
			}
		})
	}
}

func TestBase64urld(t *testing.T) {
	testCases := []struct {
		query       string
		input       any
		expected    []any
		expectedErr string
	}{
		{
			query:    "@base64urld",
			input:    "aGVsbG8=",
			expected: []any{"hello"},
		},
		{
			query:    "@base64urld",
			input:    "aGVsbG8",
			expected: []any{"hello"},
		},
		{
			query:    "@base64urld",
			input:    "aGVsbG8=trailing",
			expected: []any{"hello"},
		},
		{
			query:    "@base64urld",
			input:    "Pj8-Pw==",
			expected: []any{">?>?"},
		},
		{
			query:    "@base64urld",
			input:    "Pj8-Pw",
			expected: []any{">?>?"},
		},
		{
			query:    "@base64urld",
			input:    "Pj8-Pz8_",
			expected: []any{">?>???"},
		},
		{
			query:    `format("base64urld")`,
			input:    "Pj8-Pw==",
			expected: []any{">?>?"},
		},
		{
			query:       "@base64urld",
			input:       ":",
			expectedErr: `@base64urld cannot be applied to ":": illegal base64 data at input byte 0`,
		},
		{
			query:       "@base64urld",
			input:       "Pj8+Pw==",
			expectedErr: `@base64urld cannot be applied to "Pj8+Pw==": illegal base64 data at input byte 3`,
		},
		{
			query:       "@base64urld",
			input:       "A",
			expectedErr: `@base64urld cannot be applied to "A": illegal base64 data at input byte 0`,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%s/%s", tc.query, tc.input), func(t *testing.T) {
			query, err := gojq.Parse(tc.query)
			if err != nil {
				t.Fatal(err)
			}
			iter := query.Run(tc.input)
			var got []any
			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					if tc.expectedErr == "" {
						t.Fatalf("unexpected error: %v", err)
					}
					if err.Error() != tc.expectedErr {
						t.Fatalf("expected error: %q, got: %q", tc.expectedErr, err.Error())
					}
					return
				}
				got = append(got, v)
			}
			if tc.expectedErr != "" {
				t.Fatalf("expected error: %q, got: %v", tc.expectedErr, got)
			}
			if fmt.Sprint(got) != fmt.Sprint(tc.expected) {
				t.Fatalf("expected: %v, got: %v", tc.expected, got)
			}
		})
	}
}

func TestBase64url_BinaryRoundTrip(t *testing.T) {
	binaryStr := string([]byte{0x00, 0xff, 0xef, 0x01, 0xfe, 0xfd})
	query, err := gojq.Parse("@base64url | @base64urld")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(binaryStr)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected value")
	}
	if err, ok := v.(error); ok {
		t.Fatal(err)
	}
	if got, expected := v.(string), binaryStr; got != expected {
		t.Fatalf("expected: %q, got: %q", expected, got)
	}
}

