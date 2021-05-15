package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

func ExampleWithInputIter() {
	query, err := gojq.Parse("reduce inputs as $x (0; . + $x)")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(gojq.NewIter(1, 2, 3, 4, 5)),
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
	// 15
}
