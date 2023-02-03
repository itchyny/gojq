package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

type moduleLoader struct{}

func (*moduleLoader) LoadModule(name string) (*gojq.Query, error) {
	switch name {
	case "module1":
		return gojq.Parse(`
			module { name: "module1", test: 42 };
			import "module2" as foo;
			def f: foo::f;
		`)
	case "module2":
		return gojq.Parse(`
			def f: .foo;
		`)
	}
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
	input := map[string]any{"foo": 42}
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
