package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"runtime"
	"strings"

	"github.com/alecthomas/participle/lexer"
	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"github.com/mattn/go-runewidth"

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
		fmt.Fprintf(cli.errStream, "%s: invalid query: %s\n", name, arg)
		cli.printQueryParseError(arg, err)
		return exitCodeErr
	}
	var buf bytes.Buffer
	dec := json.NewDecoder(io.TeeReader(cli.inStream, &buf))
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
		v, err = gojq.Run(query, v)
		if err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
		if err := cli.printValue(v); err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
	}
	return exitCodeOK
}

func (cli *cli) printQueryParseError(query string, err error) {
	if err, ok := err.(*lexer.Error); ok {
		lines := strings.Split(query, "\n")
		if 0 < err.Pos.Line && err.Pos.Line <= len(lines) {
			line, col := []rune(lines[err.Pos.Line-1]), err.Pos.Column
			// somehow participle reports invalid column
			for _, prefix := range []string{"unexpected", "unexpected token"} {
				var r rune
				if _, err := fmt.Sscanf(err.Message, prefix+` "%c"`, &r); err == nil {
					c := col
					for 1 < c && (len(line) < c || line[c-1] != r) {
						c--
					}
					if c > 1 {
						col = c
						break
					}
				}
			}
			fmt.Fprintf(cli.errStream, "    %s\n%s  %s\n", string(line), strings.Repeat(" ", 3+col)+"^", err.Message)
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

func (cli *cli) printValue(v interface{}) error {
	if v == struct{}{} {
		return nil
	}
	if c, ok := v.(chan interface{}); ok {
		for x := range c {
			if err, ok := x.(error); ok {
				return err
			}
			if err := cli.printValue(x); err != nil {
				return err
			}
		}
		return nil
	}
	xs, err := jsonFormatter().Marshal(v)
	cli.outStream.Write(xs)
	cli.outStream.Write([]byte{'\n'})
	return err
}

func jsonFormatter() *prettyjson.Formatter {
	f := prettyjson.NewFormatter()
	f.StringColor = color.New(color.FgGreen)
	f.BoolColor = color.New(color.FgYellow)
	f.NumberColor = color.New(color.FgCyan)
	f.NullColor = color.New(color.FgHiBlack)
	return f
}
