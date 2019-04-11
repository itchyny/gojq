package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"

	"github.com/itchyny/gojq"
)

const name = "gojq"

const version = "0.0.0"

var revision = "HEAD"

const (
	exitCodeOK = iota
	exitCodeErr
)

type cli struct {
	inStream  io.Reader
	outStream io.Writer
	errStream io.Writer
}

func (cli *cli) run(args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(cli.errStream)
	fs.Usage = func() {
		fs.SetOutput(cli.outStream)
		fmt.Fprintf(cli.outStream, `%[1]s - Go implementation of jq

Version: %s (rev: %s/%s)

Synopsis:
    %% echo '{"foo": 128}' | %[1]s '.foo'

Options:
`, name, version, revision, runtime.Version())
		fs.PrintDefaults()
	}
	var showVersion bool
	fs.BoolVar(&showVersion, "v", false, "print version")
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return exitCodeOK
		}
		return exitCodeErr
	}
	if showVersion {
		fmt.Fprintf(cli.outStream, "%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return exitCodeOK
	}
	args = fs.Args()
	var arg string
	if len(args) == 0 {
		arg = "."
	} else if len(args) == 1 {
		arg = args[0]
	} else {
		fmt.Fprintf(cli.errStream, "%s: too many arguments\n", name)
		return exitCodeErr
	}
	parser := gojq.NewParser()
	query, err := parser.Parse(arg)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s: %s\n", name, err, arg)
		return exitCodeErr
	}
	var v interface{}
	if err := json.NewDecoder(cli.inStream).Decode(&v); err != nil {
		fmt.Fprintf(cli.errStream, "%s: invalid json: %s\n", name, err)
		return exitCodeErr
	}
	v, err = gojq.Run(query, v)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	enc := json.NewEncoder(cli.outStream)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	return exitCodeOK
}
