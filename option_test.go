package gojq_test

import (
	"fmt"
	"log"

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
