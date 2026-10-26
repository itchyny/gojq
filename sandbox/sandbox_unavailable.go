//go:build !gojq_sandbox

package sandbox

import "context"

// Run evaluates a jq query inside the wasm sandbox. This binary was built
// without the gojq_sandbox build tag, so it always returns ErrUnavailable.
func Run(ctx context.Context, query string, input any, opts *Options) ([]any, error) {
	return nil, ErrUnavailable
}
