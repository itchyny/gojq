package gojq_test

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"sync"
	"testing"

	"github.com/itchyny/gojq"
)

func TestWithFormat(t *testing.T) {
	testCases := []struct {
		name     string
		query    string
		input    any
		expected []any
		format   func(any) any
	}{
		{
			name:  "filter alone",
			query: "10 | @double",
			input: nil,
			expected: []any{
				20,
			},
			format: func(v any) any {
				if n, ok := v.(int); ok {
					return n * 2
				}
				return v
			},
		},
		{
			name:  "string interpolation",
			query: `"world" | @goprint "hello \(.)"`,
			input: nil,
			expected: []any{
				"hello world",
			},
			format: func(v any) any {
				return fmt.Sprint(v)
			},
		},
		{
			name:  "issue 271 example with array interpolation",
			query: `"abcde" | split("") | @goprint "goprint: \(.)"`,
			input: nil,
			expected: []any{
				"goprint: [a b c d e]",
			},
			format: func(v any) any {
				return fmt.Sprint(v)
			},
		},
		{
			name:  "direct format function call",
			query: `"world" | format("goprint")`,
			input: nil,
			expected: []any{
				"world",
			},
			format: func(v any) any {
				return fmt.Sprint(v)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := gojq.Parse(tc.query)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			code, err := gojq.Compile(query, gojq.WithFormat("double", tc.format), gojq.WithFormat("goprint", tc.format))
			if err != nil {
				t.Fatalf("compile error: %v", err)
			}
			iter := code.Run(tc.input)
			var got []any
			for {
				v, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := v.(error); ok {
					t.Fatalf("run error: %v", err)
				}
				got = append(got, v)
			}
			if !reflect.DeepEqual(got, tc.expected) {
				t.Fatalf("expected %#v, got %#v", tc.expected, got)
			}
		})
	}
}

func TestWithFormatLeadingAt(t *testing.T) {
	query, err := gojq.Parse(`"hello" | @quote "val: \(.)"`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, gojq.WithFormat("@quote", func(v any) any {
		return fmt.Sprintf("'%v'", v)
	}))
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected value")
	}
	if expected := "val: 'hello'"; v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestWithEscaper(t *testing.T) {
	query, err := gojq.Parse(`"abcde" | split("") | @goprint "goprint: \(.)"`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithEscaper("goprint", func(x any) any {
			return fmt.Sprint(x)
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected value")
	}
	if expected := "goprint: [a b c d e]"; v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestWithFormatError(t *testing.T) {
	query, err := gojq.Parse(`"test" | @bad`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, gojq.WithFormat("bad", func(v any) any {
		return errors.New("custom formatting failed")
	}))
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected error")
	}
	errVal, ok := v.(error)
	if !ok {
		t.Fatalf("expected error type, got %#v", v)
	}
	if expected := "custom formatting failed"; errVal.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, errVal.Error())
	}

	// Verify try operator catches the format error
	queryTry, err := gojq.Parse(`"test" | @bad?`)
	if err != nil {
		t.Fatal(err)
	}
	codeTry, err := gojq.Compile(queryTry, gojq.WithFormat("bad", func(v any) any {
		return errors.New("custom formatting failed")
	}))
	if err != nil {
		t.Fatal(err)
	}
	iterTry := codeTry.Run(nil)
	if _, ok := iterTry.Next(); ok {
		t.Fatal("expected no values when caught by try operator")
	}
}

func TestWithFormatInterpolationError(t *testing.T) {
	query, err := gojq.Parse(`"test" | @bad "output: \(.)"`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, gojq.WithFormat("bad", func(v any) any {
		return errors.New("interpolation formatting failed")
	}))
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected error")
	}
	errVal, ok := v.(error)
	if !ok {
		t.Fatalf("expected error type, got %#v", v)
	}
	if expected := "interpolation formatting failed"; errVal.Error() != expected {
		t.Fatalf("expected error %q, got %q", expected, errVal.Error())
	}
}

func TestRegisterFormat(t *testing.T) {
	formatName := "testglobalfmt"
	gojq.RegisterFormat(formatName, func(v any) any {
		return fmt.Sprintf("<<< %v >>>", v)
	})
	defer gojq.UnregisterFormat(formatName)

	query, err := gojq.Parse(`"hello" | @testglobalfmt "msg: \(.)", (. | @testglobalfmt), format("testglobalfmt")`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	var got []any
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			t.Fatalf("run error: %v", err)
		}
		got = append(got, v)
	}
	expected := []any{
		"msg: <<< hello >>>",
		"<<< hello >>>",
		"<<< hello >>>",
	}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %#v, got %#v", expected, got)
	}
}

func TestRegisterEscaper(t *testing.T) {
	escaperName := "testglobalesc"
	gojq.RegisterEscaper("@"+escaperName, func(v any) any {
		return fmt.Sprintf("[[ %v ]]", v)
	})
	defer gojq.UnregisterEscaper(escaperName)

	query, err := gojq.Parse(`"foo" | @testglobalesc "result: \(.)"`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected value")
	}
	if expected := "result: [[ foo ]]"; v != expected {
		t.Fatalf("expected %q, got %q", expected, v)
	}
}

func TestUnregisterFormat(t *testing.T) {
	formatName := "testtempfmt"
	gojq.RegisterFormat(formatName, func(v any) any {
		return fmt.Sprint(v)
	})
	gojq.UnregisterFormat(formatName)

	query, err := gojq.Parse(`"test" | format("testtempfmt")`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("expected error")
	}
	errVal, ok := v.(error)
	if !ok {
		t.Fatalf("expected error, got %#v", v)
	}
	if expected := "format not defined: @testtempfmt"; errVal.Error() != expected {
		t.Fatalf("expected %q, got %q", expected, errVal.Error())
	}
}

func TestUserDefinedFormattingFilter(t *testing.T) {
	t.Run("with string interpolation", func(t *testing.T) {
		query, err := gojq.Parse(`def upcase: ascii_upcase; "world" | @upcase "hello \(.)"`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected value")
		}
		if expected := "hello WORLD"; v != expected {
			t.Fatalf("expected %q, got %q", expected, v)
		}
	})

	t.Run("filter alone", func(t *testing.T) {
		query, err := gojq.Parse(`def upcase: ascii_upcase; "world" | @upcase`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected value")
		}
		if expected := "WORLD"; v != expected {
			t.Fatalf("expected %q, got %q", expected, v)
		}
	})

	t.Run("scoped nested function", func(t *testing.T) {
		query, err := gojq.Parse(`
			def outer:
				def inner: "nested:" + .;
				"val" | @inner "res: \(.)";
			outer
		`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected value")
		}
		if expected := "res: nested:val"; v != expected {
			t.Fatalf("expected %q, got %q", expected, v)
		}
	})
}

func TestBuiltinFunctionAsFormatFilter(t *testing.T) {
	t.Run("ascii_upcase with interpolation", func(t *testing.T) {
		query, err := gojq.Parse(`"world" | @ascii_upcase "hello \(.)"`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected value")
		}
		if expected := "hello WORLD"; v != expected {
			t.Fatalf("expected %q, got %q", expected, v)
		}
	})

	t.Run("length format filter", func(t *testing.T) {
		query, err := gojq.Parse(`[1, 2, 3] | @length`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected value")
		}
		if expected := 3; v != expected {
			t.Fatalf("expected %d, got %v", expected, v)
		}
	})
}

func TestUndefinedFormatHandling(t *testing.T) {
	t.Run("undefined format runtime error", func(t *testing.T) {
		query, err := gojq.Parse(`"abc" | @undefinedfmt`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		v, ok := iter.Next()
		if !ok {
			t.Fatal("expected error")
		}
		errVal, ok := v.(error)
		if !ok {
			t.Fatalf("expected error, got %#v", v)
		}
		if expected := "format not defined: @undefinedfmt"; errVal.Error() != expected {
			t.Fatalf("expected %q, got %q", expected, errVal.Error())
		}
	})

	t.Run("undefined format with optional operator", func(t *testing.T) {
		query, err := gojq.Parse(`"abc" | @undefinedfmt?`)
		if err != nil {
			t.Fatal(err)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			t.Fatal(err)
		}
		iter := code.Run(nil)
		if _, ok := iter.Next(); ok {
			t.Fatal("expected no outputs")
		}
	})
}

func TestFormatValidationPanics(t *testing.T) {
	panics := func(fn func()) (panicked bool) {
		defer func() {
			if recover() != nil {
				panicked = true
			}
		}()
		fn()
		return false
	}

	if !panics(func() { gojq.WithFormat("", func(v any) any { return v }) }) {
		t.Fatal("expected panic on empty format name")
	}
	if !panics(func() { gojq.WithFormat("@", func(v any) any { return v }) }) {
		t.Fatal("expected panic on '@' format name")
	}
	if !panics(func() { gojq.WithFormat("test", nil) }) {
		t.Fatal("expected panic on nil function")
	}
	if !panics(func() { gojq.RegisterFormat("", func(v any) any { return v }) }) {
		t.Fatal("expected panic on empty RegisterFormat name")
	}
	if !panics(func() { gojq.RegisterFormat("test", nil) }) {
		t.Fatal("expected panic on nil RegisterFormat function")
	}
}

func TestWithFormatConcurrency(t *testing.T) {
	query, err := gojq.Parse(`.[1] | @calc "result: \(.)"`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, gojq.WithFormat("calc", func(v any) any {
		if n, ok := v.(int); ok {
			return fmt.Sprint(n * 10)
		}
		return fmt.Sprint(v)
	}))
	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			iter := code.Run([]any{id, 5})
			v, ok := iter.Next()
			if !ok {
				t.Errorf("worker %d: expected result", id)
				return
			}
			expected := "result: 50"
			if v != expected {
				t.Errorf("worker %d: expected %q, got %q", id, expected, v)
			}
		}(i)
	}
	wg.Wait()
}

func ExampleWithEscaper() {
	query, err := gojq.Parse(`"abcde" | split("") | @goprint "goprint: \(.)"`)
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithEscaper("goprint", func(x any) any {
			return fmt.Sprint(x)
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Println(v)
	}

	// Output:
	// goprint: [a b c d e]
}

func ExampleWithFormat() {
	query, err := gojq.Parse(`"world" | @shout "hello \(. | @shout)"`)
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithFormat("shout", func(x any) any {
			return fmt.Sprintf("%v!", x)
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Println(v)
	}

	// Output:
	// hello world!!
}
