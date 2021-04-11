package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

type intIter struct {
	values []int
	index  int
}

func newIntIter(xs ...int) *intIter {
	return &intIter{xs, 0}
}

func (iter *intIter) Next() (interface{}, bool) {
	if len(iter.values) == iter.index {
		return nil, false
	}
	v := iter.values[iter.index]
	iter.index++
	return v, true
}

func ExampleWithInputIter() {
	query, err := gojq.Parse("reduce inputs as $x (0; . + $x)")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(newIntIter(1, 2, 3, 4, 5)),
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
