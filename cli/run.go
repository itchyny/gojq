package cli

import "os"

// Run gojq.
func Run() int {
	return (&cli{
		inStream:  os.Stdin,
		outStream: os.Stdout,
		errStream: os.Stderr,
	}).run(os.Args[1:])
}
