package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"github.com/mattn/go-runewidth"

	"github.com/itchyny/gojq"
)

const name = "gojq"

const version = "0.2.0"

var revision = "HEAD"

const (
	exitCodeOK = iota
	exitCodeErr
)

type cli struct {
	inStream  io.Reader
	outStream io.Writer
	errStream io.Writer

	outputRaw bool
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
	fs.BoolVar(&cli.outputRaw, "r", false, "output raw string")
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
	} else {
		arg = strings.TrimSpace(args[0])
		args = args[1:]
	}
	query, err := gojq.Parse(arg)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: invalid query: %s\n", name, arg)
		cli.printParseError(arg, err)
		return exitCodeErr
	}
	if len(args) == 0 {
		return cli.process(cli.inStream, query)
	}
	for _, arg := range args {
		if exitCode := cli.processFile(arg, query); exitCode != exitCodeOK {
			return exitCode
		}
	}
	return exitCodeOK
}

func (cli *cli) printParseError(query string, err error) {
	if err, ok := err.(*lexer.Error); ok {
		lines := strings.Split(query, "\n")
		if 0 < err.Pos.Line && err.Pos.Line <= len(lines) {
			fmt.Fprintf(
				cli.errStream, "    %s\n%s  %s\n", lines[err.Pos.Line-1],
				strings.Repeat(" ", 3+err.Pos.Column)+"^", err.Message)
		}
	}
}

func (cli *cli) processFile(fname string, query *gojq.Query) int {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	defer f.Close()
	return cli.process(f, query)
}

func (cli *cli) process(in io.Reader, query *gojq.Query) int {
	var buf bytes.Buffer
	dec := json.NewDecoder(io.TeeReader(in, &buf))
	for {
		buf.Reset()
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if buf.String() == "" {
				return exitCodeOK
			}
			fmt.Fprintf(cli.errStream, "%s: invalid json: %s\n", name, err)
			cli.printJSONError(buf.String(), err)
			return exitCodeErr
		}
		if err := cli.printValue(query.Run(v)); err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
	}
}

func (cli *cli) printJSONError(input string, err error) {
	if err.Error() == "unexpected EOF" {
		lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
		line := lines[len(lines)-1]
		fmt.Fprintf(cli.errStream, "    %s\n%s\n", line, strings.Repeat(" ", 4+runewidth.StringWidth(line))+"^")
	} else if err, ok := err.(*json.SyntaxError); ok {
		var s strings.Builder
		var i, j int
		for _, r := range input {
			i += len([]byte(string(r)))
			if i <= int(err.Offset) {
				j += runewidth.RuneWidth(r)
			}
			if r == '\n' || r == '\r' {
				if i == int(err.Offset) {
					j++
					break
				} else if i > int(err.Offset) {
					break
				} else {
					j = 0
					s.Reset()
				}
			} else {
				s.WriteRune(r)
			}
		}
		fmt.Fprintf(cli.errStream, "    %s\n%s\n", s.String(), strings.Repeat(" ", 3+j)+"^")
	}
}

func (cli *cli) printValue(v <-chan interface{}) error {
	for x := range v {
		if err, ok := x.(error); ok {
			return err
		}
		if cli.outputRaw {
			if _, ok := x.(string); ok {
				fmt.Fprintln(cli.outStream, x)
				continue
			}
		}
		xs, err := jsonFormatter().Marshal(x)
		if err != nil {
			return err
		}
		cli.outStream.Write(xs)
		cli.outStream.Write([]byte{'\n'})
	}
	return nil
}

func jsonFormatter() *prettyjson.Formatter {
	f := prettyjson.NewFormatter()
	f.StringColor = color.New(color.FgGreen)
	f.BoolColor = color.New(color.FgYellow)
	f.NumberColor = color.New(color.FgCyan)
	f.NullColor = color.New(color.FgHiBlack)
	return f
}
