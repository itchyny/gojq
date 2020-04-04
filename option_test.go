package gojq_test

import (
	"log"
	"reflect"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
)

func TestWithModuleLoaderError(t *testing.T) {
	query, err := gojq.Parse(`
		import "module1" as m;
		m::f
	`)
	if err != nil {
		log.Fatalln(err)
	}
	_, err = gojq.Compile(query)
	if got, expected := err.Error(), `cannot load module: "module1"`; got != expected {
		t.Errorf("expected: %v, got: %v", expected, got)
	}

	query, err = gojq.Parse("modulemeta")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run("m")
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		err := v.(error)
		if got, expected := err.Error(), `cannot load module: "m"`; got != expected {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
		break
	}
}

func TestWithModuleLoader_modulemeta(t *testing.T) {
	query, err := gojq.Parse(`
		"module1" | modulemeta
	`)
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithModuleLoader(&moduleLoader{}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := map[string]interface{}{
			"deps": []interface{}{
				map[string]interface{}{
					"relpath": "module2",
					"as":      "foo",
					"is_data": false,
				},
			},
			"name": "module1",
			"test": 42,
		}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestWithEnvironLoader(t *testing.T) {
	query, err := gojq.Parse("env")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithEnvironLoader(func() []string {
			return []string{"foo=42", "bar=128"}
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		expected := map[string]interface{}{"foo": "42", "bar": "128"}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %#v, got: %#v", expected, got)
		}
	}
}

func TestWithEnvironLoaderEmpty(t *testing.T) {
	query, err := gojq.Parse("env")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := map[string]interface{}{}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestWithVariablesError0(t *testing.T) {
	query, err := gojq.Parse(".")
	if err != nil {
		log.Fatalln(err)
	}
	_, err = gojq.Compile(
		query,
		gojq.WithVariables([]string{"x"}),
	)
	if got, expected := err.Error(), "invalid variable name: x"; got != expected {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
}

func TestWithVariablesError1(t *testing.T) {
	query, err := gojq.Parse(".")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithVariables([]string{"$x"}),
	)
	iter := code.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if got, expected := err.Error(), "variable defined but not bound: $x"; got != expected {
				t.Errorf("expected: %v, got: %v", expected, got)
			}
		}
	}
}

func TestWithVariablesError2(t *testing.T) {
	query, err := gojq.Parse(".")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithVariables([]string{"$x"}),
	)
	iter := code.Run(nil, 1, 2)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if got, expected := err.Error(), "too many variable values provided"; got != expected {
				t.Errorf("expected: %v, got: %v", expected, got)
			}
		}
	}
}

func TestWithInputIter(t *testing.T) {
	query, err := gojq.Parse("range(10) | input")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(
			newTestInputIter(strings.NewReader("1 2 3 4 5")),
		),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil)
	n := 1
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := "break"; err.Error() != expected {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
			break
		}
		if v != n {
			t.Errorf("expected: %v, got: %v", n, v)
		}
		n++
	}
}
