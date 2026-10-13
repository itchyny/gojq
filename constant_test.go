package gojq_test

import (
	"reflect"
	"sync"
	"testing"

	"github.com/itchyny/gojq"
)

func TestConstantCollectionsIndependentResults(t *testing.T) {
	for _, src := range []string{
		`{}`, `{"count": 1}`, `[]`, `[1, {"value": [2]}]`,
		`if . then {"count": 1} else {} end`,
		`[range(2) | {"count": 1}] | .[]`,
	} {
		t.Run(src, func(t *testing.T) {
			query, err := gojq.Parse(src)
			if err != nil {
				t.Fatal(err)
			}
			code, err := gojq.Compile(query)
			if err != nil {
				t.Fatal(err)
			}
			baseline, err := gojq.Compile(query)
			if err != nil {
				t.Fatal(err)
			}
			want, _ := baseline.Run(true).Next()
			iter := code.Run(true)
			first, ok := iter.Next()
			if !ok {
				t.Fatal("missing result")
			}
			switch first := first.(type) {
			case map[string]any:
				first["changed"] = true
			case []any:
				if len(first) > 0 {
					first[0] = "changed"
					first[1].(map[string]any)["value"].([]any)[0] = "changed"
				}
			}
			if next, ok := iter.Next(); ok && !reflect.DeepEqual(next, want) {
				t.Fatalf("mutation leaked into next result: %#v", next)
			}
			got, _ := code.Run(true).Next()
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("mutation leaked across runs: got %#v, want %#v", got, want)
			}
		})
	}
}

func TestConstantObjectConcurrentRuns(t *testing.T) {
	query, err := gojq.Parse(`{"nested": [{"count": 1}]}`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	// Obtain all outputs before starting writers, so a broken implementation
	// produces a race rather than a concurrent map iteration/write fatal error.
	objects := make([]map[string]any, 16)
	for i := range objects {
		v, _ := code.Run(nil).Next()
		objects[i] = v.(map[string]any)["nested"].([]any)[0].(map[string]any)
	}
	var wg sync.WaitGroup
	for i, object := range objects {
		wg.Add(1)
		go func() {
			defer wg.Done()
			object["count"] = i
		}()
	}
	wg.Wait()
	for i, object := range objects {
		if object["count"] != i {
			t.Fatalf("output %d was modified by another goroutine: %#v", i, object)
		}
	}
}

func TestConstantObjectIndependentRuns(t *testing.T) {
	query, err := gojq.Parse(`{"count": 1, "nested": [{"value": 2}]}`)
	if err != nil {
		t.Fatal(err)
	}
	code, err := gojq.Compile(query)
	if err != nil {
		t.Fatal(err)
	}
	first, ok := code.Run(nil).Next()
	if !ok {
		t.Fatal("missing first result")
	}
	object := first.(map[string]any)
	object["count"] = 99
	object["nested"].([]any)[0].(map[string]any)["value"] = 99
	second, ok := code.Run(nil).Next()
	want := map[string]any{"count": 1, "nested": []any{map[string]any{"value": 2}}}
	if !ok || !reflect.DeepEqual(second, want) {
		t.Fatalf("mutation leaked into another run: got %#v, want %#v", second, want)
	}
}
