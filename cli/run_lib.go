package cli

import (
	"io"
)

// RunLib allows use as library where caller sets streams and params programmatically
func RunLib(inStream io.Reader, outStream io.Writer, errStream io.Writer, opts []string) int {
	return (&cli{
		inStream:  inStream,
		outStream: outStream,
		errStream: errStream,
	}).run(opts)
}
