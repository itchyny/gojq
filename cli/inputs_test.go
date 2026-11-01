package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/iotest"
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

func TestJSONInputIterRetainsBufferedInput(t *testing.T) {
	largeValue := `"` + strings.Repeat("a", 20*1024) + `"`
	readError := func(iter inputIter) error {
		for {
			v, ok := iter.Next()
			if !ok {
				return nil
			}
			if err, ok := v.(error); ok {
				return err
			}
		}
	}
	for _, tc := range []struct {
		name  string
		input string
		line  int
		text  string
	}{
		{"large value", largeValue + "\n\"\\q\"\n", 2, `"\q"`},
		{"unexpected EOF", largeValue + "\n[1", 2, "[1"},
		{"multiple buffer resets", strings.Repeat("0\n", 10000) + largeValue + "\n[1", 10002, "[1"},
		{"multibyte value", `"` + strings.Repeat("世界", 6000) + "\"\n\"\\q\"\n", 2, `"\q"`},
	} {
		for _, mode := range []struct {
			name    string
			newIter func(io.Reader, string) inputIter
		}{
			{"normal", newJSONInputIter},
			{"stream", newStreamInputIter},
		} {
			t.Run(tc.name+"/"+mode.name, func(t *testing.T) {
				// A pipe should report the same error context as seekable input.
				wantErr := readError(mode.newIter(strings.NewReader(tc.input), "test.json"))
				wantLine := fmt.Sprintf("\n    %d | %s\n", tc.line, tc.text)
				if wantErr == nil || !strings.Contains(wantErr.Error(), wantLine) {
					t.Fatalf("seekable input: got %v, want error line %q", wantErr, wantLine)
				}
				gotErr := readError(mode.newIter(newStringReader(tc.input), "test.json"))
				if gotErr == nil || gotErr.Error() != wantErr.Error() {
					t.Errorf("pipe input: got error: %v\nwant error: %v", gotErr, wantErr)
				}
			})
		}
	}
}

func TestJSONInputIterRecoverAfterSyntaxError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		input string
		want  []any
	}{
		{"unicode", "\"a\\u263\"\n\"def\"\n", []any{"error", "def"}},
		{"same line", "\"a\\q\" \"discarded\"\n\"def\"\n", []any{"error", "def"}},
		{"CRLF", "[1,]\r\n\"def\"\r\n", []any{"error", "def"}},
		{"newline error", "n\n\"def\"\n", []any{"error", "def"}},
		{"multiple errors", "\"a\\q\"\n\"first\"\n[1,]\n\"second\"\n", []any{"error", "first", "error", "second"}},
		{"number precision", "[1,]\n9007199254740993\n", []any{"error", json.Number("9007199254740993")}},
		{"large line", "\"a\\q" + strings.Repeat("x", 20*1024) + "\"\n\"def\"\n", []any{"error", "def"}},
		{"no newline", "\"a\\q\" \"discarded\"", []any{"error"}},
	} {
		for _, r := range []io.Reader{strings.NewReader(tc.input), newStringReader(tc.input), iotest.OneByteReader(strings.NewReader(tc.input))} {
			t.Run(fmt.Sprintf("%s_%T", tc.name, r), func(t *testing.T) {
				iter := &queryInputIter{newJSONInputIter(r, "test.json")}
				var got []any
				for v, ok := iter.Next(); ok; v, ok = iter.Next() {
					if _, ok := v.(error); ok {
						v = "error"
					}
					got = append(got, v)
				}
				if !reflect.DeepEqual(got, tc.want) {
					t.Errorf("got %#v, want %#v", got, tc.want)
				}
			})
		}
	}
}

func TestJSONInputIterRecoveryErrorContext(t *testing.T) {
	input := `"` + strings.Repeat("x", 20*1024) + "\"\n\"a\\q\"\n\"valid\"\n[1,]\n"
	for _, r := range []io.Reader{strings.NewReader(input), newStringReader(input)} {
		t.Run(fmt.Sprintf("%T", r), func(t *testing.T) {
			iter := &queryInputIter{newJSONInputIter(r, "test.json")}
			var got []string
			for v, ok := iter.Next(); ok; v, ok = iter.Next() {
				if err, ok := v.(error); ok {
					got = append(got, err.Error())
				}
			}
			want := []string{
				"invalid json: test.json:2\n    2 | \"a\\q\"\n           ^  invalid character 'q' in string escape code",
				"invalid json: test.json:4\n    4 | [1,]\n           ^  invalid character ']' looking for beginning of value",
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("got errors %q, want %q", got, want)
			}
		})
	}
}

func TestJSONInputIterIOError(t *testing.T) {
	want := errors.New("read error")
	iter := &queryInputIter{newJSONInputIter(iotest.ErrReader(want), "test.json")}
	v, ok := iter.Next()
	if err, isErr := v.(*jsonParseError); !ok || !isErr || err.err != want {
		t.Fatalf("got (%v, %v), want read error", v, ok)
	}
	if v, ok := iter.Next(); ok {
		t.Fatalf("got (%v, %v) after read error", v, ok)
	}
}

func TestJSONInputIterRecoveryBoundsBuffer(t *testing.T) {
	const count = 4000
	input := strings.Repeat("\"a\\q\"\n", count) + "\"def\"\n"
	jsonIter := newJSONInputIter(newStringReader(input), "test.json").(*jsonInputIter)
	iter := &queryInputIter{jsonIter}
	var firstErr, lastErr error
	var firstMessage string
	var errors int
	var got []any
	for v, ok := iter.Next(); ok; v, ok = iter.Next() {
		if err, ok := v.(error); ok {
			if firstErr == nil {
				firstErr, firstMessage = err, err.Error()
			}
			lastErr = err
			errors++
		} else {
			got = append(got, v)
		}
		if n := jsonIter.ir.buf.Len(); n > 20*1024 {
			t.Fatalf("retained %d bytes after %d syntax errors", n, errors)
		}
	}
	if errors != count || !reflect.DeepEqual(got, []any{"def"}) {
		t.Fatalf("got %d errors and values %#v", errors, got)
	}
	if firstErr.Error() != firstMessage || !strings.Contains(lastErr.Error(), "test.json:4000\n    4000 | \"a\\q\"\n") {
		t.Errorf("error contexts changed: first %q, last %q", firstErr, lastErr)
	}
}

func TestJSONInputIterCloseAfterSyntaxError(t *testing.T) {
	iter := &queryInputIter{newJSONInputIter(strings.NewReader("[1,]\n\"def\"\n"), "test.json")}
	if v, ok := iter.Next(); !ok {
		t.Fatalf("got (%v, %v), want syntax error", v, ok)
	}
	if err := iter.Close(); err != nil {
		t.Fatal(err)
	}
	if v, ok := iter.Next(); ok {
		t.Fatalf("got (%v, %v) after close", v, ok)
	}
}

func TestCliRunRecoverJSONFiles(t *testing.T) {
	dir := t.TempDir()
	first, second := filepath.Join(dir, "first.json"), filepath.Join(dir, "second.json")
	if err := os.WriteFile(first, []byte("\"a\\u263\"\n\"first\"\n[1,]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(second, []byte("\"second\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
		err  bool
	}{
		{[]string{"-nr", `recurse(try inputs catch "error") | select(.)`, first, second}, "error\nfirst\nerror\nsecond\n", false},
		{[]string{".", first, second}, "\"second\"\n", true},
	} {
		var out, errOut strings.Builder
		c := cli{inStream: strings.NewReader(""), outStream: &out, errStream: &errOut}
		code := c.run(tc.args)
		if (code != 0) != tc.err || out.String() != tc.want || (errOut.Len() != 0) != tc.err {
			t.Errorf("args %q: got code %d, stdout %q, stderr %q", tc.args, code, out.String(), errOut.String())
		}
	}
}

func TestCliRunJSONSyntaxErrorStopsInput(t *testing.T) {
	var out, errOut strings.Builder
	c := cli{inStream: strings.NewReader("\"a\\q\"\n\"discarded\"\n"), outStream: &out, errStream: &errOut}
	code := c.run([]string{"."})
	if code == 0 || out.Len() != 0 || !strings.Contains(errOut.String(), "invalid json: <stdin>") {
		t.Errorf("got code %d, stdout %q, stderr %q", code, out.String(), errOut.String())
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
