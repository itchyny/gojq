package gojq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"testing"
	"time"

	"github.com/itchyny/gojq"
)

func ExampleQuery_Run() {
	query, err := gojq.Parse(".foo | ..")
	if err != nil {
		log.Fatalln(err)
	}
	input := map[string]interface{}{"foo": []interface{}{1, 2, 3}}
	iter := query.Run(input)
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
	// []interface {}{1, 2, 3}
	// 1
	// 2
	// 3
}

func ExampleQuery_RunWithContext() {
	query, err := gojq.Parse("def f: f; f")
	if err != nil {
		log.Fatalln(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	iter := query.RunWithContext(ctx, nil)
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

func TestQueryRun_NumericTypes(t *testing.T) {
	query, err := gojq.Parse(".[] > 1")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run([]interface{}{
		int64(2), int32(2), int16(2), int8(2), uint64(2), uint32(2), uint16(2), uint8(2),
		^uint(0), int64(math.MaxInt64), uint64(math.MaxUint64), uint32(math.MaxUint32),
		new(big.Int).SetUint64(math.MaxUint64), new(big.Int).SetUint64(math.MaxUint32),
		json.Number(fmt.Sprint(uint64(math.MaxInt64))), json.Number(fmt.Sprint(uint64(math.MaxInt32))),
		float64(2.0), float32(2.0),
	})
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			t.Fatal(err)
		}
		if expected := true; expected != v {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
	}
}
