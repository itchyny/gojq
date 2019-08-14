package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/participle/lexer"
	"github.com/jessevdk/go-flags"
	"github.com/mattn/go-runewidth"

	"github.com/itchyny/gojq"
)

const name = "gojq"

const version = "0.5.0"

var revision = "HEAD"

const (
	exitCodeOK = iota
	exitCodeErr
)

type cli struct {
	inStream  io.Reader
	outStream io.Writer
	errStream io.Writer

	outputCompact bool
	outputRaw     bool
	inputRaw      bool
	inputSlurp    bool
}

type flagopts struct {
	OutputCompact bool   `short:"c" long:"compact-output" description:"compact output"`
	OutputRaw     bool   `short:"r" long:"raw-output" description:"output raw strings"`
	InputNull     bool   `short:"n" long:"null-input" description:"use null as input value"`
	InputRaw      bool   `short:"R" long:"raw-input" description:"read input as raw strings"`
	InputSlurp    bool   `short:"s" long:"slurp" description:"read all inputs into an array"`
	FromFile      string `short:"f" long:"from-file" description:"load query from file"`
	Version       bool   `short:"v" long:"version" description:"print version"`
}

func (cli *cli) run(args []string) int {
	var opts flagopts
	args, err := flags.NewParser(
		&opts, flags.HelpFlag|flags.PassDoubleDash,
	).ParseArgs(args)
	if err != nil {
		if err, ok := err.(*flags.Error); ok && err.Type == flags.ErrHelp {
			fmt.Fprintf(cli.outStream, `%[1]s - Go implementation of jq

Version: %s (rev: %s/%s)

Synopsis:
    %% echo '{"foo": 128}' | %[1]s '.foo'

`,
				name, version, revision, runtime.Version())
			fmt.Fprintln(cli.outStream, err.Error())
			return exitCodeOK
		}
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	if opts.Version {
		fmt.Fprintf(cli.outStream, "%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return exitCodeOK
	}
	cli.outputCompact, cli.outputRaw = opts.OutputCompact, opts.OutputRaw
	cli.inputRaw, cli.inputSlurp = opts.InputRaw, opts.InputSlurp
	var arg, fname string
	if opts.FromFile != "" {
		src, err := ioutil.ReadFile(opts.FromFile)
		if err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
		arg, fname = string(src), opts.FromFile
	} else if len(args) == 0 {
		arg = "."
	} else {
		arg, fname = strings.TrimSpace(args[0]), "<arg>"
		args = args[1:]
	}
	query, err := gojq.Parse(arg)
	if err != nil {
		cli.printParseError(fname, arg, err)
		return exitCodeErr
	}
	if opts.InputNull {
		cli.inputRaw, cli.inputSlurp = false, false
		return cli.process("<null>", bytes.NewReader([]byte("null")), query)
	}
	if len(args) == 0 {
		return cli.process("<stdin>", cli.inStream, query)
	}
	for _, arg := range args {
		if exitCode := cli.processFile(arg, query); exitCode != exitCodeOK {
			return exitCode
		}
	}
	return exitCodeOK
}

func (cli *cli) printParseError(fname, query string, err error) {
	if err, ok := err.(*lexer.Error); ok {
		lines := strings.Split(query, "\n")
		if 0 < err.Pos.Line && err.Pos.Line <= len(lines) {
			var prefix string
			if len(lines) <= 1 && fname == "<arg>" {
				fname = query
			} else {
				fname += fmt.Sprintf(":%d", err.Pos.Line)
				prefix = fmt.Sprintf("%d | ", err.Pos.Line)
			}
			fmt.Fprintf(cli.errStream, "%s: invalid query: %s\n", name, fname)
			fmt.Fprintf(
				cli.errStream, "    %s%s\n%s  %s\n", prefix, lines[err.Pos.Line-1],
				strings.Repeat(" ", 3+err.Pos.Column+len(prefix))+"^", err.Message)
			return
		}
	}
	fmt.Fprintf(cli.errStream, "%s: invalid query: %s\n", name, query)
}

func (cli *cli) processFile(fname string, query *gojq.Query) int {
	f, err := os.Open(fname)
	if err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	defer f.Close()
	return cli.process(fname, f, query)
}

func (cli *cli) process(fname string, in io.Reader, query *gojq.Query) int {
	if cli.inputRaw {
		return cli.processRaw(fname, in, query)
	}
	return cli.processJSON(fname, in, query)
}

func (cli *cli) processRaw(fname string, in io.Reader, query *gojq.Query) int {
	if cli.inputSlurp {
		xs, err := ioutil.ReadAll(in)
		if err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
		if err := cli.printValue(query.Run(string(xs))); err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
		return exitCodeOK
	}
	s := bufio.NewScanner(in)
	exitCode := exitCodeOK
	for s.Scan() {
		if err := cli.printValue(query.Run(s.Text())); err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			exitCode = exitCodeErr
		}
	}
	if err := s.Err(); err != nil {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		return exitCodeErr
	}
	return exitCode
}

func (cli *cli) processJSON(fname string, in io.Reader, query *gojq.Query) int {
	var buf bytes.Buffer
	dec := json.NewDecoder(io.TeeReader(in, &buf))
	var vs []interface{}
	for {
		var v interface{}
		if err := dec.Decode(&v); err != nil {
			if err == io.EOF {
				if cli.inputSlurp {
					if err := cli.printValue(query.Run(vs)); err != nil {
						fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
						return exitCodeErr
					}
				}
				return exitCodeOK
			}
			fmt.Fprintf(cli.errStream, "%s: invalid json: %s\n", name, fname)
			cli.printJSONError(fname, buf.String(), err)
			return exitCodeErr
		}
		if cli.inputSlurp {
			vs = append(vs, v)
			continue
		}
		if err := cli.printValue(query.Run(v)); err != nil {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
			return exitCodeErr
		}
	}
}

func (cli *cli) printJSONError(fname, input string, err error) {
	if err.Error() == "unexpected EOF" {
		lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
		line := toValidUTF8(strings.TrimRight(lines[len(lines)-1], "\r"))
		fmt.Fprintf(cli.errStream, "    %s\n%s  %s\n", line, strings.Repeat(" ", 4+runewidth.StringWidth(line))+"^", err)
	} else if err, ok := err.(*json.SyntaxError); ok {
		var s strings.Builder
		var i, j int
		for _, r := range toValidUTF8(input) {
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
		fmt.Fprintf(cli.errStream, "    %s\n%s  %s\n", s.String(), strings.Repeat(" ", 3+j)+"^", err)
	}
}

func toValidUTF8(s string) string {
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}

func (cli *cli) printValue(v gojq.Iter) error {
	m := cli.createMarshaler()
	for {
		m, outStream := m, cli.outStream
		x, ok := v.Next()
		if !ok {
			break
		}
		switch v := x.(type) {
		case error:
			return v
		case [2]interface{}:
			if s, ok := v[0].(string); ok {
				outStream = cli.errStream
				compact := cli.outputCompact
				cli.outputCompact = true
				m = cli.createMarshaler()
				cli.outputCompact = compact
				if s == "STDERR:" {
					x = v[1]
				}
			}
		}
		xs, err := m.Marshal(x)
		if err != nil {
			return err
		}
		outStream.Write(xs)
		outStream.Write([]byte{'\n'})
	}
	return nil
}

func (cli *cli) createMarshaler() marshaler {
	f := jsonFormatter()
	if cli.outputCompact {
		f.Indent = 0
		f.Newline = ""
	}
	if cli.outputRaw {
		return &rawMarshaler{f}
	}
	return f
}
