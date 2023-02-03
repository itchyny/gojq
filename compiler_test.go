package gojq_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/itchyny/gojq"
)

func ExampleCompile() {
	query, err := gojq.Parse(".[] | .foo")
	if err != nil {
		log.Fatalln(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		log.Fatalln(err)
	}
	iter := code.Run([]any{
		nil,
		"string",
		42,
		[]any{"foo"},
		map[string]any{"foo": 42},
	})
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			fmt.Println(err)
			continue
		}
		fmt.Printf("%#v\n", v)
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
	input := map[string]any{"foo": 42}
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
	query, err := gojq.Parse("def f: f; f, f")
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
			fmt.Println(err)
			continue
		}
		_ = v
	}

	// Output:
	// context deadline exceeded
}

func TestCodeCompile_OptimizeConstants(t *testing.T) {
	query, err := gojq.Parse(`[1,{foo:2,"bar":+3},[-4]]`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 3; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := []any{
			1, map[string]any{"foo": 2, "bar": 3}, []any{-4},
		}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestCodeCompile_OptimizeIndexSlice(t *testing.T) {
	query, err := gojq.Parse(`.foo."bar".["baz"].[-1]."".[0:1]`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 8; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if got != nil {
			t.Errorf("expected: %v, got: %v", nil, got)
		}
	}
}

func TestCodeCompile_OptimizeIndexSliceAssign(t *testing.T) {
	query, err := gojq.Parse(`.foo."bar".["baz"].[0]."".[0:1] = [0]`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 8; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := map[string]any{
			"foo": map[string]any{
				"bar": map[string]any{
					"baz": []any{map[string]any{"": []any{0}}},
				},
			},
		}; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestCodeCompile_OptimizeTailRec_While(t *testing.T) {
	query, err := gojq.Parse("0 | while(. < 10; . + 1)")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 48; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	op1 := codes.Index(2).Elem().FieldByName("op")
	op2 := codes.Index(21).Elem().FieldByName("op") // test jump of call _while
	if got, expected := *(*int)(unsafe.Pointer(op2.UnsafeAddr())),
		*(*int)(unsafe.Pointer(op1.UnsafeAddr())); expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	n := 0
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if !reflect.DeepEqual(got, n) {
			t.Errorf("expected: %v, got: %v", n, got)
		}
		n++
	}
	if expected := 10; n != expected {
		t.Errorf("expected: %v, got: %v", expected, n)
	}
}

func TestCodeCompile_OptimizeTailRec_CallRec(t *testing.T) {
	query, err := gojq.Parse(`
		def f: . as $x | $x, (if $x < 3 then $x + 1 | f else empty end), $x; f
	`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 48; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	op1 := codes.Index(39).Elem().FieldByName("op") // callrec f
	op2 := codes.Index(38).Elem().FieldByName("op") // call _add/2
	if got, expected := *(*int)(unsafe.Pointer(op2.UnsafeAddr()))+1,
		*(*int)(unsafe.Pointer(op1.UnsafeAddr())); expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
}

func TestCodeCompile_OptimizeJumps(t *testing.T) {
	query, err := gojq.Parse("def f: 1; def g: 2; def h: 3; f")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	codes := reflect.ValueOf(code).Elem().FieldByName("codes")
	if got, expected := codes.Len(), 15; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	v := codes.Index(1).Elem().FieldByName("v")
	if got, expected := *(*any)(unsafe.Pointer(v.UnsafeAddr())), 13; expected != got {
		t.Errorf("expected: %v, got: %v", expected, got)
	}
	iter := code.Run(nil)
	for {
		got, ok := iter.Next()
		if !ok {
			break
		}
		if expected := 1; !reflect.DeepEqual(got, expected) {
			t.Errorf("expected: %v, got: %v", expected, got)
		}
	}
}

func TestCodeRun_Race(t *testing.T) {
	query, err := gojq.Parse("range(10)")
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			iter := code.Run(nil)
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

func BenchmarkCompile(b *testing.B) {
	cnt, err := os.ReadFile("builtin.jq")
	if err != nil {
		b.Fatal(err)
	}
	query, err := gojq.Parse(string(cnt))
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
		_, err := gojq.Compile(
			query,
			gojq.WithInputIter(gojq.NewIter()),
		)
		if err != nil {
			b.Fatal(err)
		}
	}
}
