//go:build gojq_sandbox

package sandbox

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestRun(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	got, err := Run(ctx, ".foo | add", map[string]any{"foo": []any{1, 2, 3}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || fmt.Sprint(got[0]) != "6" {
		t.Errorf("expected [6], got %v", got)
	}
}

func TestRunMemoryLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	for _, query := range []string{
		"[range(10000000)] | length",
		"def f: [f]; f",
		// exhausts memory inside a builtin (regexp), where the guest's Go
		// runtime aborts with a fatal error rather than a clean one; the
		// sandbox must still report ErrMemoryLimit, not the runtime dump.
		`("a" * 1000000) | [match(".";"g")] | length`,
	} {
		if _, err := Run(ctx, query, nil, nil); !errors.Is(err, ErrMemoryLimit) {
			t.Errorf("%s: expected ErrMemoryLimit, got %v", query, err)
		}
	}
}

func TestRunTimeLimit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := Run(ctx, "repeat(empty)", nil, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestRunQueryError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var queryErr *QueryError
	if _, err := Run(ctx, ".foo + 1", map[string]any{"foo": "bar"}, nil); !errors.As(err, &queryErr) {
		t.Errorf("expected a QueryError, got %v", err)
	}
}

func TestRunConcurrent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			got, err := Run(ctx, ".+1", float64(i), nil)
			if err != nil {
				t.Errorf("run %d: %v", i, err)
				return
			}
			if len(got) != 1 || got[0] != float64(i+1) {
				t.Errorf("run %d: got %v", i, got)
			}
		}(i)
	}
	wg.Wait()
}
