package gojq_test

import (
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"testing"

	"github.com/itchyny/gojq"
)

func TestWithModuleLoaderError(t *testing.T) {
	query, err := gojq.Parse(`
		import "module1" as m;
		m::f
	`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = gojq.Compile(query)
	if got, expected := err.Error(), `cannot load module: "module1"`; got != expected {
		t.Errorf("expected: %v, got: %v", expected, got)
	}

	query, err = gojq.Parse("modulemeta")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithModuleLoader(&moduleLoader{}),
	)
	if err != nil {
		t.Fatal(err)
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

type moduleLoaderJSON struct{}

func (*moduleLoaderJSON) LoadJSON(name string) (interface{}, error) {
	switch name {
	case "module1":
		return []interface{}{1.0, 42, json.Number("123")}, nil
	}
	return nil, fmt.Errorf("module not found: %q", name)
}

func TestWithModuleLoader_JSON(t *testing.T) {
	query, err := gojq.Parse(`
		import "module1" as $m;
		[$m, $m[1]*$m[2]*1000000000000]
	`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithModuleLoader(&moduleLoaderJSON{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := []interface{}{
			[]interface{}{1.0, 42, 123},
			big.NewInt(5166000000000000),
		}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

type moduleLoaderInitModules struct{}

func (*moduleLoaderInitModules) LoadInitModules() ([]*gojq.Query, error) {
	query, err := gojq.Parse(`
		def f: 42;
		def g: f * f;
	`)
	if err != nil {
		return nil, err
	}
	return []*gojq.Query{query}, nil
}

func TestWithModuleLoader_LoadInitModules(t *testing.T) {
	query, err := gojq.Parse("g")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithModuleLoader(&moduleLoaderInitModules{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := 42 * 42; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestWithEnvironLoader(t *testing.T) {
	query, err := gojq.Parse("env")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithEnvironLoader(func() []string {
			return []string{"foo=42", "bar=128"}
		}),
	)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
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
		t.Fatal(err)
	}
	for _, name := range []string{"", "$", "_x", "x", "$0x", "$$", "$x ", " $x"} {
		_, err = gojq.Compile(
			query,
			gojq.WithVariables([]string{name}),
		)
		if err == nil {
			t.Fatalf("%q is not a valid variable name", name)
		}
		if got, expected := err.Error(), "invalid variable name: "+name; got != expected {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestWithVariablesError1(t *testing.T) {
	query, err := gojq.Parse(".")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithVariables([]string{"$x"}),
	)
	if err != nil {
		t.Fatal(err)
	}
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
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithVariables([]string{"$x"}),
	)
	if err != nil {
		t.Fatal(err)
	}
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

func TestWithFunction(t *testing.T) {
	options := []gojq.CompilerOption{
		gojq.WithFunction("f", 0, 0, func(x interface{}, _ []interface{}) interface{} {
			if x, ok := x.(int); ok {
				return x * 2
			}
			return fmt.Errorf("f cannot be applied to: %v", x)
		}),
		gojq.WithFunction("g", 1, 1, func(x interface{}, xs []interface{}) interface{} {
			if x, ok := x.(int); ok {
				if y, ok := xs[0].(int); ok {
					return x + y
				}
			}
			return fmt.Errorf("g cannot be applied to: %v, %v", x, xs)
		}),
	}
	query, err := gojq.Parse(".[] | f | g(3)")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run([]interface{}{0, 1, 2, 3, 4})
	n := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := n*2 + 3; v != expected {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
		n++
	}
	query, err = gojq.Parse(
		`("f/0", "f/1", "g/0", "g/1") as $f | builtins | any(. == $f)`,
	)
	if err != nil {
		t.Fatal(err)
	}
	code, err = gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter = code.Run(nil)
	n = 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := n == 0 || n == 3; v != expected {
			t.Errorf("expected: %v, got: %v (n = %d)", expected, v, n)
		}
		n++
	}
}

func TestWithFunctionDuplicateName(t *testing.T) {
	options := []gojq.CompilerOption{
		gojq.WithFunction("f", 0, 0, func(x interface{}, _ []interface{}) interface{} {
			if x, ok := x.(int); ok {
				return x * 2
			}
			return fmt.Errorf("f cannot be applied to: %v", x)
		}),
		gojq.WithFunction("f", 1, 1, func(x interface{}, xs []interface{}) interface{} {
			if x, ok := x.(int); ok {
				if y, ok := xs[0].(int); ok {
					return x + y
				}
			}
			return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
		}),
		gojq.WithFunction("f", 0, 0, func(x interface{}, _ []interface{}) interface{} {
			if x, ok := x.(int); ok {
				return x * 4
			}
			return fmt.Errorf("f cannot be applied to: %v", x)
		}),
		gojq.WithFunction("f", 2, 2, func(x interface{}, xs []interface{}) interface{} {
			if x, ok := x.(int); ok {
				if y, ok := xs[0].(int); ok {
					if z, ok := xs[1].(int); ok {
						return x * y * z
					}
				}
			}
			return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
		}),
	}
	query, err := gojq.Parse(".[] | f | f(3) | f(4; 5)")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run([]interface{}{0, 1, 2, 3, 4})
	n := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := (n*4 + 3) * 4 * 5; v != expected {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
		n++
	}
	query, err = gojq.Parse(
		`("f/0", "f/1", "f/2", "f/3") as $f | builtins | any(. == $f)`,
	)
	if err != nil {
		t.Fatal(err)
	}
	code, err = gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter = code.Run(nil)
	n = 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := n < 3; v != expected {
			t.Errorf("expected: %v, got: %v (n = %d)", expected, v, n)
		}
		n++
	}
}

func TestWithFunctionMultipleArities(t *testing.T) {
	options := []gojq.CompilerOption{
		gojq.WithFunction("f", 0, 4, func(x interface{}, xs []interface{}) interface{} {
			if x, ok := x.(int); ok {
				x *= 2
				for _, y := range xs {
					if y, ok := y.(int); ok {
						x += y
					}
				}
				return x
			}
			return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
		}),
		gojq.WithFunction("f", 2, 3, func(x interface{}, xs []interface{}) interface{} {
			if x, ok := x.(int); ok {
				for _, y := range xs {
					if y, ok := y.(int); ok {
						x *= y
					}
				}
				return x
			}
			return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
		}),
		gojq.WithFunction("g", 0, 30, func(x interface{}, xs []interface{}) interface{} {
			return nil
		}),
	}
	query, err := gojq.Parse(".[] | f | f(1) | f(2; 3) | f(4; 5; 6) | f(7; 8; 9; 10)")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run([]interface{}{0, 1, 2, 3, 4})
	n := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := (((n*2*2+1)*2*3)*4*5*6)*2 + 7 + 8 + 9 + 10; v != expected {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
		n++
	}
	query, err = gojq.Parse(
		`("f/0", "f/1", "f/2", "f/3", "f/4", "f/5") as $f | builtins | any(. == $f)`,
	)
	if err != nil {
		t.Fatal(err)
	}
	code, err = gojq.Compile(query, options...)
	if err != nil {
		t.Fatal(err)
	}
	iter = code.Run(nil)
	n = 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := n < 5; v != expected {
			t.Errorf("expected: %v, got: %v (n = %d)", expected, v, n)
		}
		n++
	}
}

type valueError struct {
	v interface{}
}

func (err *valueError) Error() string {
	return "error: " + fmt.Sprint(err.v)
}

func (err *valueError) Value() interface{} {
	return err.v
}

func TestWithFunctionValueError(t *testing.T) {
	expected := map[string]interface{}{"foo": 42}
	query, err := gojq.Parse("try f catch .")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query,
		gojq.WithFunction("f", 0, 0, func(x interface{}, _ []interface{}) interface{} {
			return &valueError{expected}
		}),
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if !reflect.DeepEqual(v, expected) {
			t.Errorf("expected: %#v, got: %#v", expected, v)
		}
	}
}

type moduleLoader2 struct{}

func (*moduleLoader2) LoadModule(name string) (*gojq.Query, error) {
	switch name {
	case "module1":
		return gojq.Parse(`
			def g: def h: f * 3; h * 4;
		`)
	}
	return nil, fmt.Errorf("module not found: %q", name)
}

func TestWithFunctionWithModuleLoader(t *testing.T) {
	query, err := gojq.Parse(`include "module1"; .[] | g`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query,
		gojq.WithFunction("f", 0, 0, func(x interface{}, _ []interface{}) interface{} {
			if x, ok := x.(int); ok {
				return x * 2
			}
			return fmt.Errorf("f cannot be applied to: %v", x)
		}),
		gojq.WithModuleLoader(&moduleLoader2{}),
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := code.Run([]interface{}{0, 1, 2, 3, 4})
	n := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if expected := n * 2 * 3 * 4; v != expected {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
		n++
	}
}

func TestWithInputIter(t *testing.T) {
	query, err := gojq.Parse("range(10) | input")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(newIntIter(1, 2, 3, 4, 5)),
	)
	if err != nil {
		t.Fatal(err)
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
