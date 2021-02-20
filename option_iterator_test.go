package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

func ExampleWithIterator() {
	query, err := gojq.Parse("f | . * 2")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithIterator("f", 0, 0, func(x interface{}, xs []interface{}) interface{} {
			i := 0
			return gojq.IterFn(func() (interface{}, bool) {
				if i > 2 {
					return nil, false
				}
				i++
				return i - 1, true
			})
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
	// 0
	// 2
	// 4
}
