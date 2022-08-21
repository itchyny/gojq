package cli

import (
	"bytes"
	"io"
	"os"
)

// Config specifies configuration to run the gojq CLI with.
type Config struct {
	// Input and output streams for the CLI.
	//
	// If Stdin is nil, an empty stdin will be used.
	// If Stdout or Stderr are nil, that output stream will be discarded.
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Run the gojq CLI with the provided arguments,
// and return the exit code.
//
// The arguments must not contain os.Args[0].
func (cfg *Config) Run(args []string) (exitCode int) {
	cli := &cli{
		inStream:  cfg.Stdin,
		outStream: cfg.Stdout,
		errStream: cfg.Stderr,
	}
	if cli.inStream == nil {
		cli.inStream = bytes.NewReader(nil)
	}
	if cli.outStream == nil {
		cli.outStream = io.Discard
	}
	if cli.errStream == nil {
		cli.errStream = io.Discard
	}

	return cli.run(args)
}

// Run gojq.
func Run() int {
	return (&Config{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}).Run(os.Args[1:])
}
