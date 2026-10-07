//go:build gojq_sandbox

package sandbox

import (
	"context"
	"testing"
)

func BenchmarkRun(b *testing.B) {
	ctx := context.Background()
	input := map[string]any{"foo": []any{1.0, 2.0, 3.0}}
	if _, err := Run(ctx, ".foo | add", input, nil); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Run(ctx, ".foo | add", input, nil); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkRunParallel(b *testing.B) {
	ctx := context.Background()
	input := map[string]any{"foo": []any{1.0, 2.0, 3.0}}
	if _, err := Run(ctx, ".foo | add", input, nil); err != nil {
		b.Fatal(err)
	}
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if _, err := Run(ctx, ".foo | add", input, nil); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkRunLargeInput(b *testing.B) {
	ctx := context.Background()
	items := make([]any, 10000)
	for i := range items {
		items[i] = map[string]any{"name": "item", "value": float64(i)}
	}
	input := map[string]any{"items": items}
	if _, err := Run(ctx, "[.items[].value] | add", input, nil); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Run(ctx, "[.items[].value] | add", input, nil); err != nil {
			b.Fatal(err)
		}
	}
}
