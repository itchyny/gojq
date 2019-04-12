package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/alecthomas/participle/lexer"

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
		arg = strings.TrimSpace(args[0])
	} else {
		fmt.Fprintf(cli.errStream, "%s: too many arguments\n", name)
		return exitCodeErr
	}
	parser := gojq.NewParser()
	query, err := parser.Parse(arg)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: invalid query: %q\n", name, arg)
		cli.printQueryParseError(arg, err)
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

func (cli *cli) printQueryParseError(query string, err error) {
	if err, ok := err.(*lexer.Error); ok {
		lines := strings.Split(query, "\n")
		if 0 < err.Pos.Line && err.Pos.Line <= len(lines) {
			fmt.Fprintf(cli.errStream, "    %s\n%s\n", lines[err.Pos.Line-1], strings.Repeat(" ", 3+err.Pos.Column)+"^")
		}
	}
}
