package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

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
