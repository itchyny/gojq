package gojq_test

import (
	"fmt"
	"log"

	"github.com/itchyny/gojq"
)

type testInputIter struct {
	n int
}

func (i *testInputIter) Next() (interface{}, bool) {
	if i.n >= 5 {
		return nil, false
	}
	i.n++
	return i.n, true
}

func ExampleWithInputIter() {
	query, err := gojq.Parse("input")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(&testInputIter{}),
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
	// 1
}
