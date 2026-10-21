//go:build gojq_sandbox

package sandbox

import (
	"bytes"
	"compress/gzip"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
)

//go:generate make -C .. build-sandbox

//go:embed gojq_guest.wasm.gz
var guestWasmGz []byte

// guestWasm returns the guest module, decompressed once on first use. The
// module is embedded gzip-compressed to keep the committed file small (about
// 1.6 MB rather than 6 MB); compress/gzip is in the standard library, so this
// adds no dependency, and go get already transfers the module zip compressed.
var guestWasm = sync.OnceValue(func() []byte {
	r, err := gzip.NewReader(bytes.NewReader(guestWasmGz))
	if err != nil {
		panic("sandbox: cannot read the embedded guest module: " + err.Error())
	}
	b, err := io.ReadAll(r)
	if err != nil {
		panic("sandbox: cannot read the embedded guest module: " + err.Error())
	}
	return b
})

// A runtime is created once per distinct memory limit and reused; the
// compiled machine code is shared between them through compilationCache, so
// the module is compiled once per process. Module instantiation per Run is
// cheap, compilation is not.
var (
	runtimesMu       sync.Mutex
	runtimes         = map[uint32]cachedRuntime{}
	compilationCache = wazero.NewCompilationCache()
)

type cachedRuntime struct {
	runtime  wazero.Runtime
	compiled wazero.CompiledModule
}

func compiledFor(ctx context.Context, pages uint32) (cachedRuntime, error) {
	runtimesMu.Lock()
	defer runtimesMu.Unlock()
	if c, ok := runtimes[pages]; ok {
		return c, nil
	}
	rt := wazero.NewRuntimeWithConfig(ctx, wazero.NewRuntimeConfig().
		WithMemoryLimitPages(pages).
		WithCloseOnContextDone(true).
		WithCompilationCache(compilationCache).
		WithDebugInfoEnabled(false))
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)
	compiled, err := rt.CompileModule(ctx, guestWasm())
	if err != nil {
		_ = rt.Close(ctx)
		return cachedRuntime{}, err
	}
	c := cachedRuntime{runtime: rt, compiled: compiled}
	runtimes[pages] = c
	return c, nil
}

// Run evaluates a jq query on an input value inside the wasm sandbox and
// returns the query outputs. The input is encoded to JSON for the sandbox and
// the outputs are decoded from it, so values follow encoding/json conventions
// (numbers are float64). The context bounds the execution time; the memory
// cap in opts bounds everything the query can allocate. Run is safe for
// concurrent use.
func Run(ctx context.Context, query string, input any, opts *Options) ([]any, error) {
	limit := int64(0)
	if opts != nil {
		limit = opts.MemoryLimit
	}
	if limit <= 0 {
		limit = DefaultMemoryLimit
	}
	pages := uint32((limit + 65535) / 65536)

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	c, err := compiledFor(ctx, pages)
	if err != nil {
		return nil, err
	}

	var stdout, stderr bytes.Buffer
	mod, err := c.runtime.InstantiateModule(ctx, c.compiled, wazero.NewModuleConfig().
		WithName("").
		WithArgs("gojq-guest", query).
		WithStdin(bytes.NewReader(inputJSON)).
		WithStdout(&stdout).
		WithStderr(&stderr))
	if mod != nil {
		// Close the instance to free its linear memory now, rather than
		// leaving it to the runtime and the garbage collector; a 64 MiB
		// cap held until the next GC lets a few concurrent runs add up.
		_ = mod.Close(context.Background())
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		switch {
		case ctx.Err() != nil:
			return nil, ctx.Err()
		case isOutOfMemory(message):
			return nil, ErrMemoryLimit
		default:
			var exitErr *sys.ExitError
			if errors.As(err, &exitErr) && message != "" {
				return nil, &QueryError{Message: firstLine(message)}
			}
			return nil, err
		}
	}

	var outputs []any
	dec := json.NewDecoder(&stdout)
	for dec.More() {
		var v any
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		outputs = append(outputs, v)
	}
	return outputs, nil
}

// isOutOfMemory reports whether the guest stderr indicates the query exhausted
// the memory cap. When the guest's Go runtime cannot grow past the cap it aborts
// with a fatal error ("out of memory", "arena already initialized", and similar)
// rather than a normal query error, which is printed on a single line.
func isOutOfMemory(message string) bool {
	return strings.Contains(message, "out of memory") ||
		strings.Contains(message, "arena already initialized") ||
		strings.Contains(message, "cannot allocate memory") ||
		strings.Contains(message, "fatal error: runtime: ")
}

// firstLine returns the first line of a guest error message. A normal query
// error is a single line; this keeps a stray multi-line runtime dump from
// becoming the error string.
func firstLine(message string) string {
	if i := strings.IndexByte(message, '\n'); i >= 0 {
		return message[:i]
	}
	return message
}
