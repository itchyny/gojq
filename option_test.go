package gojq_test

import (
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/itchyny/gojq"
)

type moduleLoader struct{}

func (*moduleLoader) LoadInitModules() ([]*gojq.Module, error) {
	return nil, nil
}

func (*moduleLoader) LoadModule(name string) (*gojq.Module, error) {
	if name == "module1" {
		return gojq.ParseModule("def f: .foo;")
	}
	return nil, fmt.Errorf("module not found: %q", name)
}

func (*moduleLoader) LoadJSON(name string) (interface{}, error) {
	return nil, fmt.Errorf("module not found: %q", name)
}

func ExampleWithModuleLoader() {
	query, err := gojq.Parse(`
		import "module1" as m;
		m::f
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
	input := map[string]interface{}{"foo": 42}
	iter := code.Run(input)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Printf("%#v\n", v)
	}

	// Output:
	// 42
}

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
}

func ExampleWithEnvironLoader() {
	query, err := gojq.Parse("env | keys[]")
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
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Printf("%#v\n", v)
	}

	// Output:
	// "bar"
	// "foo"
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

func ExampleWithVariables() {
	query, err := gojq.Parse("$x * 100 + $y, $z")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithVariables([]string{
			"$x", "$y", "$z",
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run(nil, 12, 42, 128)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			log.Fatalln(err)
		}
		fmt.Printf("%#v\n", v)
	}

	// Output:
	// 1242
	// 128
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
