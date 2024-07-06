package gojq_test

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/itchyny/gojq"
)

func BenchmarkMemoryLeak(b *testing.B) {
	benchCases := []string{
		`range(.) | select(false)`,
		`range(.) | if (false) then . else empty end`,
	}
	const MB = 1024 * 1024
	memThreshold := float32(10)
	if memEnv := os.Getenv("MEM_THRESHOLD"); memEnv != "" {
		num, err := strconv.Atoi(memEnv)
		if err != nil {
			b.Fatal("MEM_THRESHOLD:", err)
		}
		memThreshold = float32(num)
	}
	memThreshold *= MB
	for _, bc := range benchCases {
		query, err := gojq.Parse(bc)
		if err != nil {
			b.Fatal(err)
		}
		b.Run(strings.ReplaceAll(bc, " ", ""), func(b *testing.B) {
			var memStats runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memStats)
			memUsage1 := float32(memStats.HeapAlloc)
			iter := query.Run(b.N)
			for {
				_, ok := iter.Next()
				if !ok {
					break
				}
			}
			runtime.ReadMemStats(&memStats)
			memUsage2 := float32(memStats.HeapAlloc)
			b.Logf("%.1f MB => %.1f MB    (%d iterations)", memUsage1/MB, memUsage2/MB, b.N)
			if memUsage2-memUsage1 > memThreshold {
				b.Errorf("MEM_THRESHOLD (%.0f MB) passed, failing...", memThreshold/MB)
			}
		})
	}
}
