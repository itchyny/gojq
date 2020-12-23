package gojq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"reflect"
	"strings"
	"sync"
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
	query, err := gojq.Parse("def f: f; f, f")
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
			fmt.Println(err)
			continue
		}
		_ = v
	}

	// Output:
	// context deadline exceeded
}

func TestQueryRun_Errors(t *testing.T) {
	query, err := gojq.Parse(".[] | error")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run([]interface{}{0, 1, 2, 3, 4})
	n := 0
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := fmt.Sprintf("error: %d", n); err.Error() != expected {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else {
			t.Errorf("errors should occur")
		}
		n++
	}
}

func TestQueryRun_ObjectError(t *testing.T) {
	query, err := gojq.Parse(".[] | {(.): 1}")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run([]interface{}{0, "x", []interface{}{}})
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := "expected a string for object key but got"; !strings.Contains(err.Error(), expected) {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else if expected := map[string]interface{}{"x": 1}; !reflect.DeepEqual(v, expected) {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
	}
}

func TestQueryRun_InvalidPathError(t *testing.T) {
	query, err := gojq.Parse(". + 1, path(. + 1)")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(0)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := "invalid path against: number (1)"; err.Error() != expected {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else if expected := 1; !reflect.DeepEqual(v, expected) {
			t.Errorf("expected: %v, got: %v", expected, v)
		}
	}
}

func TestQueryRun_NumericTypes(t *testing.T) {
	query, err := gojq.Parse(".[] != 0")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run([]interface{}{
		int64(1), int32(1), int16(1), int8(1), uint64(1), uint32(1), uint16(1), uint8(1), ^uint(0),
		int64(math.MaxInt64), int64(math.MinInt64), uint64(math.MaxUint64), uint32(math.MaxUint32),
		new(big.Int).SetUint64(math.MaxUint64), new(big.Int).SetUint64(math.MaxUint32),
		json.Number(fmt.Sprint(uint64(math.MaxInt64))), json.Number(fmt.Sprint(uint64(math.MaxInt32))),
		float64(1.0), float32(1.0),
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

func TestQueryRun_Race(t *testing.T) {
	query, err := gojq.Parse("range(10)")
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iter := query.Run(nil)
			var n int
			for {
				got, ok := iter.Next()
				if !ok {
					break
				}
				if got != n {
					t.Errorf("expected: %v, got: %v", n, got)
				}
				n++
			}
		}()
	}
	wg.Wait()
}
