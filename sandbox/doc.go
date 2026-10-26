// Package sandbox runs a jq query inside a WebAssembly sandbox with a hard
// memory cap and a context deadline. The interpreter is a prebuilt wasm
// module (gojq compiled for wasip1, see the guest directory); each Run
// executes it in a fresh instance whose linear memory is capped, so a query
// cannot allocate more memory than the caller allows, and a query failure
// cannot affect the embedding process.
//
// The sandbox is opt-in at build time: without the gojq_sandbox build tag,
// Run returns ErrUnavailable and the wasm runtime dependency is not linked.
// Build with -tags gojq_sandbox to enable it.
package sandbox

import "errors"

// DefaultMemoryLimit is the memory cap applied when Options.MemoryLimit is 0.
const DefaultMemoryLimit = 64 << 20

// Options configures a sandboxed run.
type Options struct {
	// MemoryLimit is the total linear memory the query may use, in bytes,
	// rounded up to the wasm page size (64 KiB). It bounds everything inside
	// the sandbox: the interpreter, the decoded input, and every value the
	// query builds. The default is DefaultMemoryLimit.
	//
	// The cap applies to each run independently, so N concurrent runs can use
	// up to N times this much memory; bound the number of concurrent runs to
	// keep the host within its own memory.
	MemoryLimit int64
}

// ErrUnavailable is returned by Run when the package was built without the
// gojq_sandbox build tag.
var ErrUnavailable = errors.New("sandbox: built without the gojq_sandbox build tag")

// ErrMemoryLimit is returned by Run when the query exceeds the memory cap.
var ErrMemoryLimit = errors.New("sandbox: memory limit exceeded")

// QueryError is returned by Run when the query itself fails (a parse error,
// a compile error, or a runtime error emitted by the query).
type QueryError struct {
	Message string
}

func (err *QueryError) Error() string {
	return "sandbox: " + err.Message
}
