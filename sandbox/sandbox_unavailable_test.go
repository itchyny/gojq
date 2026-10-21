//go:build !gojq_sandbox

package sandbox

import (
	"context"
	"errors"
	"testing"
)

func TestRunUnavailable(t *testing.T) {
	if _, err := Run(context.Background(), ".", nil, nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("expected ErrUnavailable, got %v", err)
	}
}
