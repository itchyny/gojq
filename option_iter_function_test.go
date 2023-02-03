package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

// Implementation of range/2 using WithIterFunction option.
type rangeIter struct {
	value, max int
}

func (iter *rangeIter) Next() (any, bool) {
	if iter.value >= iter.max {
		return nil, false
	}
	v := iter.value
	iter.value++
	return v, true
}

func ExampleWithIterFunction() {
	query, err := gojq.Parse("f(3; 7)")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithIterFunction("f", 2, 2, func(_ any, xs []any) gojq.Iter {
			if x, ok := xs[0].(int); ok {
				if y, ok := xs[1].(int); ok {
					return &rangeIter{x, y}
				}
			}
			return gojq.NewIter(fmt.Errorf("f cannot be applied to: %v", xs))
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
	// 3
	// 4
	// 5
	// 6
}
