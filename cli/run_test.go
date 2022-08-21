package cli

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func ExampleConfig() {
	var stdout, stderr bytes.Buffer
	stdin := strings.NewReader(`["foo", "bar", "baz"]`)
	code := (&Config{
		Stdin:  stdin,
		Stdout: &stdout,
		Stderr: &stderr,
	}).Run([]string{"-r", ". | .[]"})

	if code != 0 {
		log.Fatalf("exit code: got %v, expected: 0", code)
	}

	if stderr.Len() > 0 {
		log.Fatalf("stderr: got %q, expected empty", stderr.String())
	}

	fmt.Print(stdout.String())

	// Output:
	// foo
	// bar
	// baz
}

func TestConfigStdin(t *testing.T) {
	testCases := []struct {
		name   string
		stdin  io.Reader
		output string
	}{
		{
			name:   "set",
			stdin:  strings.NewReader("{}"),
			output: "{}\n",
		},
		{
			name:   "unset",
			stdin:  nil,
			output: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			code := (&Config{
				Stdin:  tc.stdin,
				Stdout: &out,
			}).Run([]string{"."})

			if code != 0 {
				t.Errorf("exit code: got %v, expected: 0", code)
			}

			if diff := cmp.Diff(tc.output, out.String()); diff != "" {
				t.Error("standard output:\n" + diff)
			}
		})
	}
}

func TestConfigStdoutUnset(t *testing.T) {
	// When Stdout is unset, we need to make sure we don't write to
	// os.Stdout.
	// To test that, replace os.Stdout with a temporary file,
	// and check that it's still empty after the program finishes.
	defer func(stdout *os.File) {
		os.Stdout = stdout
	}(os.Stdout)
	stdout, err := os.CreateTemp(t.TempDir(), "stdout")
	if err != nil {
		t.Fatalf("create temporary stdout: %v", err)
	}
	os.Stdout = stdout

	code := (&Config{
		Stdin: strings.NewReader("{}"),
	}).Run([]string{"."})

	if code != 0 {
		t.Errorf("exit code: got %v, expected: 0", code)
	}

	if err := stdout.Close(); err != nil {
		t.Fatalf("close temporary stdout: %v", err)
	}

	out, err := os.ReadFile(stdout.Name())
	if err != nil {
		t.Fatalf("read temporary stdout: %v", err)
	}

	if len(out) > 0 {
		t.Errorf("expected os.Stdout to be empty, got %q", out)
	}
}

func TestConfigStderrSet(t *testing.T) {
	var stderr bytes.Buffer
	code := (&Config{
		Stderr: &stderr,
	}).Run([]string{"--not-a-real-flag"})

	if code == 0 {
		t.Errorf("exit code: got %v, expected != 0", code)
	}

	if !strings.Contains(stderr.String(), "unknown flag") {
		t.Errorf(`stderr output: must contain "unknown flag", got %q`, stderr.String())
	}
}

func TestConfigStderrUnset(t *testing.T) {
	// When Stderr is unset, we need to make sure we don't write to
	// os.Stderr.
	// To test that, replace os.Stderr with a temporary file,
	// and check that it's still empty after the program finishes.
	defer func(stderr *os.File) {
		os.Stderr = stderr
	}(os.Stderr)
	stderr, err := os.CreateTemp(t.TempDir(), "stderr")
	if err != nil {
		t.Fatalf("create temporary stderr: %v", err)
	}
	os.Stderr = stderr

	code := new(Config).Run([]string{"--not-a-real-flag"})
	if code == 0 {
		t.Errorf("exit code: got %v, expected != 0", code)
	}

	if err := stderr.Close(); err != nil {
		t.Fatalf("close temporary stderr: %v", err)
	}

	out, err := os.ReadFile(stderr.Name())
	if err != nil {
		t.Fatalf("read temporary stderr: %v", err)
	}

	if len(out) > 0 {
		t.Errorf("expected os.Stderr to be empty, got %q", out)
	}
}
