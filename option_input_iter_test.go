package gojq_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/itchyny/gojq"
)

type testInputIter struct {
	dec *json.Decoder
}

func newTestInputIter(r io.Reader) *testInputIter {
	dec := json.NewDecoder(r)
	dec.UseNumber()
	return &testInputIter{dec: dec}
}

func (i *testInputIter) Next() (interface{}, bool) {
	var v interface{}
	if err := i.dec.Decode(&v); err != nil {
		if err == io.EOF {
			return nil, false
		}
		return err, true
	}
	return v, true
}

func ExampleWithInputIter() {
	query, err := gojq.Parse("reduce inputs as $x (0; . + $x)")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(
		query,
		gojq.WithInputIter(
			newTestInputIter(strings.NewReader("1 2 3 4 5")),
		),
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
