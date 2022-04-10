package gojq_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/big"
	"os"
	"reflect"
	"strconv"
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
			if expected := "error: " + strconv.Itoa(n); err.Error() != expected {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else {
			t.Errorf("errors should occur")
		}
		n++
	}
	if expected := 5; n != expected {
		t.Errorf("expected: %v, got: %v", expected, n)
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

func TestQueryRun_IndexError(t *testing.T) {
	query, err := gojq.Parse(".foo")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run([]interface{}{0})
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := "expected an object but got: array ([0])"; !strings.Contains(err.Error(), expected) {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else {
			t.Errorf("errors should occur")
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

func TestQueryRun_IteratorError(t *testing.T) {
	query, err := gojq.Parse(".[]")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			if expected := "cannot iterate over: null"; err.Error() != expected {
				t.Errorf("expected: %v, got: %v", expected, err)
			}
		} else {
			t.Errorf("errors should occur")
		}
	}
}

func TestQueryRun_TypePanic(t *testing.T) {
	defer func() { recover() }()
	query, err := gojq.Parse("type")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(struct{}{})
	for {
		_, ok := iter.Next()
		if !ok {
			break
		}
	}
	t.Errorf("panic should occur")
}

func TestQueryRun_Strings(t *testing.T) {
	query, err := gojq.Parse(
		"[\"\x00\\\\\", \"\x1f\\\"\", \"\n\\n\n\\(\"\\n\")\n\\n\", " +
			"\"\\/\", \"\x7f\", \"\x80\", \"\\ud83d\\ude04\" | explode[]]",
	)
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(nil)
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			t.Fatal(err)
		}
		if expected := []interface{}{0x00, int('\\'), 0x1f, int('"'), int('\n'), int('\n'), int('\n'),
			int('\n'), int('\n'), int('\n'), int('/'), 0x7f, 0xfffd, 128516}; !reflect.DeepEqual(v, expected) {
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

func TestQueryRun_Input(t *testing.T) {
	query, err := gojq.Parse("input")
	if err != nil {
		t.Fatal(err)
	}
	iter := query.Run(nil)
	v, ok := iter.Next()
	if !ok {
		t.Fatal("should emit an error but got no output")
	}
	err, expected := v.(error), "input(s)/0 is not allowed"
	if got := err.Error(); got != expected {
		t.Errorf("expected: %v, got: %v", expected, got)
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
			n := 0
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
			if expected := 10; n != expected {
				t.Errorf("expected: %v, got: %v", expected, n)
			}
		}()
	}
	wg.Wait()
}

func TestQueryString(t *testing.T) {
	cnt, err := os.ReadFile("builtin.jq")
	if err != nil {
		t.Fatal(err)
	}
	q, err := gojq.Parse(string(cnt))
	if err != nil {
		t.Fatal(err)
	}
	r, err := gojq.Parse(q.String())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(q, r) {
		t.Errorf("\n%v\n%v", q, r)
	}
}

func BenchmarkRun(b *testing.B) {
	query, err := gojq.Parse("range(1000)")
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		iter := query.Run(nil)
		for {
			_, ok := iter.Next()
			if !ok {
				break
			}
		}
	}
}

func BenchmarkParse(b *testing.B) {
	cnt, err := os.ReadFile("builtin.jq")
	if err != nil {
		b.Fatal(err)
	}
	src := string(cnt)
	for i := 0; i < b.N; i++ {
		_, err := gojq.Parse(src)
		if err != nil {
			b.Fatal(err)
		}
	}
}
