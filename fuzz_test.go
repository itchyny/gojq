package gojq

import (
	"context"
	"encoding/json"
	"errors"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

const (
	fuzzMaxAlloc       = int64(24 << 20)
	fuzzMaxStackDepth  = 4096
	fuzzContextTimeout = time.Second
	fuzzWallTimeout    = 3 * time.Second
	fuzzHeapAllowance  = uint64(256 << 20)
	fuzzMaxProgramSize = 16 << 10
	fuzzMaxJSONSize    = 1 << 20
)

// FuzzMeteredRun exercises the value meter and interpreter stack cap together.
// Large inputs are represented by compact JSON recipes so that the corpus stays
// small while native builtins still receive caller-owned arrays, strings, and
// deeply nested objects.
func FuzzMeteredRun(f *testing.F) {
	deepPath := strings.Repeat(".a", 1024)
	nestedArrays := strings.Repeat("[", 256) + "." + strings.Repeat("]", 256)

	// Infinite/deep user-function recursion.
	f.Add(`def f: f; f`, `null`)
	f.Add(`def f: [f]; f`, `null`)
	f.Add(`def f: (f, empty); f`, `null`)

	// Value bombs and conversion working sets.
	f.Add(`[range(1000000)]`, `null`)
	f.Add(`[range(1e6)] | implode`, `null`)
	f.Add(`"9" * 20000000 | tonumber`, `null`)

	// String and structural amplification.
	f.Add(`. * . * .`, `{"$fuzz":"string","n":2097152,"text":"ab"}`)
	f.Add(`. + . | . + . | . + . | . + .`, `{"$fuzz":"string","n":2097152,"text":"ab"}`)
	f.Add(nestedArrays, `null`)

	// Input-proportional native builtins.
	f.Add(`group_by(. % 97)`, `{"$fuzz":"numbers","n":200000}`)
	f.Add(`unique_by(. % 97)`, `{"$fuzz":"numbers","n":200000}`)
	f.Add(`sort_by(. % 97)`, `{"$fuzz":"numbers","n":200000}`)
	f.Add(`[.[]] | transpose`, `{"$fuzz":"matrix","rows":384,"cols":384}`)

	// Deep caller-owned input and a correspondingly deep path expression.
	f.Add(deepPath, `{"$fuzz":"deep-object","depth":1024}`)

	// A representative ordinary workload keeps successful execution in corpus.
	f.Add(`[.status.conditions[] | select(.type=="Cond0") | .status][0]`,
		`{"status":{"conditions":[{"type":"Cond0","status":"True"},{"type":"Cond1","status":"False"}]}}`)

	f.Fuzz(func(t *testing.T, program, rawInput string) {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Errorf("panic outside evaluator for program %q and input %q: %v\n%s",
					program, rawInput, recovered, debug.Stack())
			}
		}()

		// Parsing and compilation are outside the metered evaluator. Bound their
		// fuzzed text so this target measures execution rather than parser size.
		if len(program) > fuzzMaxProgramSize || len(rawInput) > fuzzMaxJSONSize {
			return
		}
		input, ok := fuzzInput(rawInput)
		if !ok {
			return
		}
		query, err := Parse(program)
		if err != nil {
			return
		}
		code, err := Compile(query)
		if err != nil {
			return
		}

		oldAlloc, oldStackDepth := MaxAlloc, MaxStackDepth
		MaxAlloc, MaxStackDepth = fuzzMaxAlloc, fuzzMaxStackDepth
		restoreLimits := true
		defer func() {
			if restoreLimits {
				MaxAlloc, MaxStackDepth = oldAlloc, oldStackDepth
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), fuzzContextTimeout)
		defer cancel()
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		baselineHeap, peakHeap := mem.HeapInuse, mem.HeapInuse
		resultc := make(chan fuzzRunResult, 1)
		go drainFuzzRun(ctx, code, input, resultc)

		wall := time.NewTimer(fuzzWallTimeout)
		defer wall.Stop()
		heapSample := time.NewTicker(25 * time.Millisecond)
		defer heapSample.Stop()

		for {
			select {
			case result := <-resultc:
				runtime.ReadMemStats(&mem)
				if mem.HeapInuse > peakHeap {
					peakHeap = mem.HeapInuse
				}
				if result.recovered != nil {
					t.Fatalf("evaluator panic for program %q and input %q: %v\n%s",
						program, rawInput, result.recovered, result.stack)
				}
				if result.badError != nil {
					t.Fatalf("unexpected runtime/OOM error for program %q and input %q: %T: %v",
						program, rawInput, result.badError, result.badError)
				}
				if peakHeap > baselineHeap && peakHeap-baselineHeap > fuzzHeapAllowance {
					t.Fatalf("metered run grew HeapInuse by %d MiB (allowance %d MiB) for program %q and input %q",
						(peakHeap-baselineHeap)>>20, fuzzHeapAllowance>>20, program, rawInput)
				}
				return
			case <-heapSample.C:
				runtime.ReadMemStats(&mem)
				if mem.HeapInuse > peakHeap {
					peakHeap = mem.HeapInuse
				}
			case <-wall.C:
				cancel()
				// A stuck evaluator may still be reading the global allocation
				// limit. Leave it enabled while the failing worker exits.
				restoreLimits = false
				t.Fatalf("metered run exceeded the %v hard wall for program %q and input %q",
					fuzzWallTimeout, program, rawInput)
			}
		}
	})
}

type fuzzRunResult struct {
	recovered any
	stack     []byte
	badError  error
}

func drainFuzzRun(ctx context.Context, code *Code, input any, resultc chan<- fuzzRunResult) {
	result := fuzzRunResult{}
	defer func() {
		if recovered := recover(); recovered != nil {
			result.recovered = recovered
			result.stack = debug.Stack()
		}
		resultc <- result
	}()
	iter := code.RunWithContext(ctx, input)
	for {
		value, ok := iter.Next()
		if !ok {
			return
		}
		if err, isError := value.(error); isError && fuzzUnexpectedRuntimeError(err) {
			result.badError = err
			return
		}
	}
}

func fuzzUnexpectedRuntimeError(err error) bool {
	// Allocation/stack limits, context cancellation, and ordinary jq errors
	// are all valid terminal outcomes. A Go runtime error is never one.
	var runtimeError runtime.Error
	if errors.As(err, &runtimeError) {
		return true
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	switch err.(type) {
	case *allocLimitError, *stackLimitError:
		return false
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{
		"runtime error:",
		"fatal error:",
		"out of memory",
		"cannot allocate memory",
		"stack overflow",
		"goroutine stack exceeds",
	} {
		if strings.Contains(message, marker) {
			return true
		}
	}
	return false
}

// fuzzInput decodes ordinary JSON or expands one of the compact, bounded input
// recipes used by the seed corpus. Mutations of the recipe fields explore sizes
// and shapes without allowing the harness itself to allocate without a ceiling.
func fuzzInput(raw string) (any, bool) {
	var input any
	if err := json.Unmarshal([]byte(raw), &input); err != nil {
		return nil, false
	}
	object, isObject := input.(map[string]any)
	if !isObject {
		return input, true
	}
	kind, _ := object["$fuzz"].(string)
	switch kind {
	case "numbers":
		n, ok := fuzzRecipeInt(object, "n", 750000)
		if !ok {
			return input, true
		}
		numbers := make([]any, n)
		for i := range numbers {
			numbers[i] = float64(i % 4096)
		}
		return numbers, true
	case "matrix":
		rows, rowsOK := fuzzRecipeInt(object, "rows", 1<<18)
		cols, colsOK := fuzzRecipeInt(object, "cols", 1<<18)
		if !rowsOK || !colsOK {
			return input, true
		}
		const maxCells = 1 << 18
		if rows > 0 && cols > maxCells/rows {
			cols = maxCells / rows
		}
		matrix := make([]any, rows)
		for row := range matrix {
			values := make([]any, cols)
			for col := range values {
				values[col] = float64((row + col) % 4096)
			}
			matrix[row] = values
		}
		return matrix, true
	case "string":
		n, ok := fuzzRecipeInt(object, "n", 8<<20)
		if !ok {
			return input, true
		}
		text, _ := object["text"].(string)
		if text == "" {
			text = "x"
		}
		repeated := strings.Repeat(text, (n+len(text)-1)/len(text))
		return repeated[:n], true
	case "deep-object":
		depth, ok := fuzzRecipeInt(object, "depth", fuzzMaxStackDepth)
		if !ok {
			return input, true
		}
		var nested any = "leaf"
		for range depth {
			nested = map[string]any{"a": nested}
		}
		return nested, true
	default:
		return input, true
	}
}

func fuzzRecipeInt(object map[string]any, key string, maximum int) (int, bool) {
	value, ok := object[key].(float64)
	if !ok {
		return 0, false
	}
	if value <= 0 {
		return 0, true
	}
	if value >= float64(maximum) {
		return maximum, true
	}
	return int(value), true
}
