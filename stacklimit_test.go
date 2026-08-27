package gojq

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func compileStackLimitQuery(t *testing.T, src string) *Code {
	t.Helper()
	query, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	code, err := Compile(query)
	if err != nil {
		t.Fatalf("Compile(%q): %v", src, err)
	}
	return code
}

func nextWithStackLimitTimeout(t *testing.T, code *Code, input any) (any, bool) {
	t.Helper()
	type result struct {
		value any
		ok    bool
	}
	resultc := make(chan result, 1)
	go func() {
		value, ok := code.Run(input).Next()
		resultc <- result{value, ok}
	}()
	select {
	case result := <-resultc:
		return result.value, result.ok
	case <-time.After(time.Second):
		t.Fatal("query did not reach the stack limit within one second")
		return nil, false
	}
}

func TestStackLimitStopsPureRecursionFast(t *testing.T) {
	defer func(oldDepth int, oldAlloc int64) {
		MaxStackDepth, MaxAlloc = oldDepth, oldAlloc
	}(MaxStackDepth, MaxAlloc)
	MaxStackDepth, MaxAlloc = 64, 0

	code := compileStackLimitQuery(t, `def f: f; f`)
	start := time.Now()
	value, ok := nextWithStackLimitTimeout(t, code, nil)
	if !ok {
		t.Fatal("pure recursion returned no error")
	}
	if _, isStackLimit := value.(*stackLimitError); !isStackLimit {
		t.Fatalf("pure recursion returned %T (%v), want *stackLimitError", value, value)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("pure recursion reached the stack limit too slowly: %v", elapsed)
	} else {
		t.Logf("pure recursion reached the depth-64 limit in %v", elapsed)
	}
}

func TestStackLimitStopsValueRecursionAndNestedForks(t *testing.T) {
	defer func(oldDepth int, oldAlloc int64) {
		MaxStackDepth, MaxAlloc = oldDepth, oldAlloc
	}(MaxStackDepth, MaxAlloc)
	MaxStackDepth, MaxAlloc = 64, 1<<20

	for _, src := range []string{
		`def f: [f]; f`,
		`def f: (f, empty); f`,
		`def f: . as $x | f; f`,
	} {
		value, ok := nextWithStackLimitTimeout(t, compileStackLimitQuery(t, src), nil)
		if !ok {
			t.Errorf("%q returned no error", src)
			continue
		}
		switch value.(type) {
		case *stackLimitError, *allocLimitError:
		default:
			t.Errorf("%q returned %T (%v), want a stack or allocation limit error", src, value, value)
		}
	}
}

func TestDefaultStackLimitBoundsBackingStorage(t *testing.T) {
	var value any
	bytesPerDepth := 2*unsafe.Sizeof(block{}) +
		unsafe.Sizeof(scopeBlock{}) +
		unsafe.Sizeof(fork{}) +
		unsafe.Sizeof(value)
	// Go slice capacity can be almost twice its length at a growth boundary.
	upperBound := 2 * uintptr(defaultMaxStackDepth) * bytesPerDepth
	if upperBound >= 2<<20 {
		t.Fatalf("default stack backing upper bound is %d bytes, want under 2 MiB", upperBound)
	}
	t.Logf("default stack backing upper bound: %d bytes", upperBound)
}

func TestStackLimitAllowsBoundedPrograms(t *testing.T) {
	defer func(oldDepth int, oldAlloc int64) {
		MaxStackDepth, MaxAlloc = oldDepth, oldAlloc
	}(MaxStackDepth, MaxAlloc)
	MaxStackDepth, MaxAlloc = defaultMaxStackDepth, 1<<20

	matcher := compileStackLimitQuery(t, `[.status.conditions[] | select(.type=="Cond0") | .status][0]`)
	workload := map[string]any{"status": map[string]any{"conditions": []any{
		map[string]any{"type": "Cond0", "status": "True"},
		map[string]any{"type": "Cond1", "status": "False"},
	}}}
	if value, ok := matcher.Run(workload).Next(); !ok || value != "True" {
		t.Fatalf("workload matcher returned %v, %v; want True, true", value, ok)
	}

	const nesting = 64
	nested := compileStackLimitQuery(t, strings.Repeat("[", nesting)+"."+strings.Repeat("]", nesting))
	value, ok := nested.Run(1.0).Next()
	if !ok {
		t.Fatal("bounded nested expression returned no value")
	}
	want := any(1.0)
	for range nesting {
		want = []any{want}
	}
	if !reflect.DeepEqual(value, want) {
		t.Fatal("bounded nested expression returned the wrong value")
	}
}

func TestStackLimitTailDepthResetsAfterReturn(t *testing.T) {
	defer func(oldDepth int, oldAlloc int64) {
		MaxStackDepth, MaxAlloc = oldDepth, oldAlloc
	}(MaxStackDepth, MaxAlloc)
	MaxStackDepth, MaxAlloc = 128, 0

	code := compileStackLimitQuery(t, `def countdown: if . > 0 then . - 1 | countdown else . end; [countdown, countdown]`)
	if value, ok := code.Run(100).Next(); !ok || fmt.Sprint(value) != "[0 0]" {
		t.Fatalf("two bounded tail-recursive calls returned %v, %v; want [0 0], true", value, ok)
	}
}

func TestStackLimitConcurrentRunsIsolated(t *testing.T) {
	defer func(oldDepth int, oldAlloc int64) {
		MaxStackDepth, MaxAlloc = oldDepth, oldAlloc
	}(MaxStackDepth, MaxAlloc)
	MaxStackDepth, MaxAlloc = 128, 0

	unbounded := compileStackLimitQuery(t, `def f: f; f`)
	bounded := compileStackLimitQuery(t, `reduce .[] as $x (0; . + $x)`)
	numbers := make([]any, 1000)
	for i := range numbers {
		numbers[i] = i
	}

	type result struct {
		value any
		ok    bool
	}
	start := make(chan struct{})
	unboundedResult := make(chan result, 1)
	boundedResult := make(chan result, 1)
	go func() {
		<-start
		value, ok := unbounded.Run(nil).Next()
		unboundedResult <- result{value, ok}
	}()
	go func() {
		<-start
		value, ok := bounded.Run(numbers).Next()
		boundedResult <- result{value, ok}
	}()
	close(start)

	select {
	case result := <-unboundedResult:
		if _, isStackLimit := result.value.(*stackLimitError); !result.ok || !isStackLimit {
			t.Errorf("unbounded concurrent run returned %T (%v), %v; want *stackLimitError, true", result.value, result.value, result.ok)
		}
	case <-time.After(time.Second):
		t.Fatal("unbounded concurrent run did not reach its stack limit")
	}
	select {
	case result := <-boundedResult:
		if !result.ok || fmt.Sprint(result.value) != "499500" {
			t.Errorf("bounded concurrent run returned %v, %v; want 499500, true", result.value, result.ok)
		}
	case <-time.After(time.Second):
		t.Fatal("bounded concurrent run did not finish")
	}
}
