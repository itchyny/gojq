package gojq_test

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"testing"
	"time"

	"github.com/itchyny/gojq"
)

func ExampleCompile() {
	query, err := gojq.Parse(".foo")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	inputs := []interface{}{
		nil,
		"string",
		42,
		[]interface{}{"foo"},
		map[string]interface{}{"foo": 42},
	}
	for _, input := range inputs {
		iter := code.Run(input)
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if err, ok := v.(error); ok {
				fmt.Printf("%s\n", err)
				break
			}
			fmt.Printf("%#v\n", v)
		}
	}

	// Output:
	// <nil>
	// expected an object but got: string ("string")
	// expected an object but got: number (42)
	// expected an object but got: array (["foo"])
	// 42
}

func ExampleCode_Run() {
	query, err := gojq.Parse(".foo")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
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

func ExampleCode_RunWithContext() {
	query, err := gojq.Parse("def f: f; f")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	iter := code.RunWithContext(ctx, nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			fmt.Printf("%s\n", err)
			return
		}
		_ = v
	}

	// Output:
	// context deadline exceeded
}

func Test_CodeCompileOptimizeConstants(t *testing.T) {
	query, err := gojq.Parse("[1,{foo:2},[3]]")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	if got, expected := reflect.ValueOf(code).Elem().FieldByName("codes").Len(), 3; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := []interface{}{
			1, map[string]interface{}{"foo": 2}, []interface{}{3},
		}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}
