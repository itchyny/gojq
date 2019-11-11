package cli

import (
	"io"
)

func RunLib(inStream io.Reader, outStream io.Writer, errStream io.Writer, opts []string) int {
	return (&cli{
		inStream:  inStream,
		outStream: outStream,
		errStream: errStream,
	}).run(opts)
}
