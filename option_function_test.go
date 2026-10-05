package gojq_test

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

func ExampleWithFunction() {
	query, err := gojq.Parse(".[] | f | f(3)")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithFunction("f", 0, 1, func(x any, xs []any) any {
			if x, ok := gojq.ToFloat64(x); ok {
				if len(xs) == 1 {
					if y, ok := gojq.ToFloat64(xs[0]); ok {
						x *= y
					} else {
						return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
					}
				} else {
					x += 2
				}
				return x
			}
			return fmt.Errorf("f cannot be applied to: %v, %v", x, xs)
		}),
	)
	if err != nil {
		log.Fatalln(err)
	}
	input := []any{0, 1, 2.5, json.Number("10000000000000000000000000000000000000000")}
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
	// 6
	// 9
	// 13.5
	// 3e+40
}
