package cli

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/itchyny/go-flags"
	"github.com/mattn/go-isatty"

	"github.com/itchyny/gojq"
)

const name = "gojq"

const version = "0.12.2"

var revision = "HEAD"

const (
	exitCodeOK = iota
	exitCodeFalsyErr
	exitCodeFlagParseErr
	exitCodeCompileErr
	exitCodeNoValueErr
	exitCodeDefaultErr
)

type cli struct {
	inStream  io.Reader
	outStream io.Writer
	errStream io.Writer

	outputCompact bool
	outputRaw     bool
	outputJoin    bool
	outputNul     bool
	outputYAML    bool
	outputIndent  *int
	outputTab     bool
	inputRaw      bool
	inputSlurp    bool
	inputStream   bool
	inputYAML     bool

	argnames  []string
	argvalues []interface{}

	outputYAMLSeparator bool
	exitCodeError       error
}

type flagopts struct {
	OutputCompact bool              `short:"c" long:"compact-output" description:"compact output"`
	OutputRaw     bool              `short:"r" long:"raw-output" description:"output raw strings"`
	OutputJoin    bool              `short:"j" long:"join-output" description:"stop printing a newline after each output"`
	OutputNul     bool              `short:"0" long:"nul-output" description:"print NUL after each output"`
	OutputColor   bool              `short:"C" long:"color-output" description:"colorize output even if piped"`
	OutputMono    bool              `short:"M" long:"monochrome-output" description:"stop colorizing output"`
	OutputYAML    bool              `long:"yaml-output" description:"output by YAML"`
	OutputIndent  *int              `long:"indent" description:"number of spaces for indentation"`
	OutputTab     bool              `long:"tab" description:"use tabs for indentation"`
	InputNull     bool              `short:"n" long:"null-input" description:"use null as input value"`
	InputRaw      bool              `short:"R" long:"raw-input" description:"read input as raw strings"`
	InputSlurp    bool              `short:"s" long:"slurp" description:"read all inputs into an array"`
	InputStream   bool              `long:"stream" description:"parse input in stream fashion"`
	InputYAML     bool              `long:"yaml-input" description:"read input as YAML"`
	FromFile      string            `short:"f" long:"from-file" description:"load query from file"`
	ModulePaths   []string          `short:"L" description:"directory to search modules from"`
	Args          map[string]string `long:"arg" description:"set variable to string value" count:"2" unquote:"false"`
	ArgsJSON      map[string]string `long:"argjson" description:"set variable to JSON value" count:"2" unquote:"false"`
	SlurpFile     map[string]string `long:"slurpfile" description:"set variable to the JSON contents of the file" count:"2" unquote:"false"`
	RawFile       map[string]string `long:"rawfile" description:"set variable to the contents of the file" count:"2" unquote:"false"`
	ExitStatus    bool              `short:"e" long:"exit-status" description:"exit 1 when the last value is false or null"`
	Version       bool              `short:"v" long:"version" description:"print version"`
}

var addDefaultModulePaths = true

func (cli *cli) run(args []string) int {
	if err := cli.runInternal(args); err != nil {
		cli.printError(err)
		if err, ok := err.(interface{ ExitCode() int }); ok {
			return err.ExitCode()
		}
		return exitCodeDefaultErr
	}
	return exitCodeOK
}

func (cli *cli) runInternal(args []string) (err error) {
	var opts flagopts
	args, err = flags.NewParser(
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
			return nil
		}
		return &flagParseError{err}
	}
	if opts.Version {
		fmt.Fprintf(cli.outStream, "%s %s (rev: %s/%s)\n", name, version, revision, runtime.Version())
		return nil
	}
	cli.outputCompact, cli.outputRaw, cli.outputJoin, cli.outputNul,
		cli.outputYAML, cli.outputIndent, cli.outputTab =
		opts.OutputCompact, opts.OutputRaw, opts.OutputJoin, opts.OutputNul,
		opts.OutputYAML, opts.OutputIndent, opts.OutputTab
	defer func(x bool) { noColor = x }(noColor)
	if opts.OutputColor || opts.OutputMono {
		noColor = opts.OutputMono
	} else if os.Getenv("NO_COLOR") != "" {
		noColor = true
	} else {
		noColor = !isTTY(cli.outStream)
	}
	if !noColor {
		if colors := os.Getenv("GOJQ_COLORS"); colors != "" {
			if err := setColors(colors); err != nil {
				return err
			}
		}
	}
	if i := cli.outputIndent; i != nil {
		if *i > 9 {
			return fmt.Errorf("too many indentation count: %d", *i)
		} else if *i < 0 {
			return fmt.Errorf("negative indentation count: %d", *i)
		}
	}
	if opts.OutputYAML && opts.OutputTab {
		return errors.New("cannot use tabs for YAML output")
	}
	cli.inputRaw, cli.inputSlurp, cli.inputStream, cli.inputYAML =
		opts.InputRaw, opts.InputSlurp, opts.InputStream, opts.InputYAML
	for k, v := range opts.Args {
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, v)
	}
	for k, v := range opts.ArgsJSON {
		val, _ := newJSONInputIter(strings.NewReader(v), "$"+k).Next()
		if err, ok := val.(error); ok {
			return err
		}
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, val)
	}
	for k, v := range opts.SlurpFile {
		val, err := slurpFile(v)
		if err != nil {
			return err
		}
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, val)
	}
	for k, v := range opts.RawFile {
		val, err := ioutil.ReadFile(v)
		if err != nil {
			return err
		}
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, string(val))
	}
	var arg, fname string
	if opts.FromFile != "" {
		src, err := ioutil.ReadFile(opts.FromFile)
		if err != nil {
			return err
		}
		arg, fname = string(src), opts.FromFile
	} else if len(args) == 0 {
		arg = "."
	} else {
		arg, fname = strings.TrimSpace(args[0]), "<arg>"
		args = args[1:]
	}
	if opts.ExitStatus {
		cli.exitCodeError = &exitCodeError{exitCodeNoValueErr}
		defer func() {
			if er, ok := err.(interface{ ExitCode() int }); !ok || er.ExitCode() == 0 {
				err = cli.exitCodeError
			}
		}()
	}
	query, err := gojq.Parse(arg)
	if err != nil {
		return &queryParseError{"query", fname, arg, err}
	}
	modulePaths := opts.ModulePaths
	if len(modulePaths) == 0 && addDefaultModulePaths {
		modulePaths = []string{"", "../lib/jq", "lib"}
		if homeDir, err := os.UserHomeDir(); err == nil {
			modulePaths[0] = filepath.Join(homeDir, ".jq")
		} else {
			modulePaths = modulePaths[1:]
		}
	}
	iter := cli.createInputIter(args)
	defer iter.Close()
	code, err := gojq.Compile(query,
		gojq.WithModuleLoader(gojq.NewModuleLoader(modulePaths)),
		gojq.WithEnvironLoader(os.Environ),
		gojq.WithVariables(cli.argnames),
		gojq.WithInputIter(iter),
	)
	if err != nil {
		if err, ok := err.(interface {
			QueryParseError() (string, string, string, error)
		}); ok {
			typ, name, query, err := err.QueryParseError()
			if _, err := os.Stat(name); os.IsNotExist(err) {
				name = fname + ":" + name
			}
			return &queryParseError{typ, name, query, err}
		}
		if err, ok := err.(interface {
			JSONParseError() (string, string, error)
		}); ok {
			fname, contents, err := err.JSONParseError()
			return &compileError{&jsonParseError{fname, contents, 0, err}}
		}
		return &compileError{err}
	}
	if opts.InputNull {
		iter = newNullInputIter()
	}
	return cli.process(iter, code)
}

func slurpFile(name string) (interface{}, error) {
	iter := newSlurpInputIter(newFilesInputIter(newJSONInputIter, []string{name}))
	defer iter.Close()
	val, _ := iter.Next()
	if err, ok := val.(error); ok {
		return nil, err
	}
	return val, nil
}

func (cli *cli) createInputIter(args []string) (iter inputIter) {
	var newIter func(io.Reader, string) inputIter
	switch {
	case cli.inputRaw:
		newIter = newRawInputIter
	case cli.inputStream:
		newIter = newStreamInputIter
	case cli.inputYAML:
		newIter = newYAMLInputIter
	default:
		newIter = newJSONInputIter
	}
	if cli.inputSlurp {
		defer func() {
			if cli.inputRaw {
				iter = newSlurpRawInputIter(iter)
			} else {
				iter = newSlurpInputIter(iter)
			}
		}()
	}
	if len(args) == 0 {
		return newIter(cli.inStream, "<stdin>")
	}
	return newFilesInputIter(newIter, args)
}

func (cli *cli) process(iter inputIter, code *gojq.Code) error {
	var err error
	for {
		v, ok := iter.Next()
		if !ok {
			return err
		}
		if er, ok := v.(error); ok {
			cli.printError(er)
			err = &emptyError{er}
			continue
		}
		if er := cli.printValues(code.Run(v, cli.argvalues...)); er != nil {
			cli.printError(er)
			err = &emptyError{er}
		}
	}
}

func (cli *cli) printValues(v gojq.Iter) error {
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
				} else {
					x = []interface{}{v[0], v[1]}
				}
			}
		}
		if cli.outputYAMLSeparator {
			outStream.Write([]byte("---\n"))
		} else {
			cli.outputYAMLSeparator = cli.outputYAML
		}
		if err := m.marshal(x, outStream); err != nil {
			return err
		}
		if cli.exitCodeError != nil {
			if x == nil || x == false {
				cli.exitCodeError = &exitCodeError{exitCodeFalsyErr}
			} else {
				cli.exitCodeError = &exitCodeError{exitCodeOK}
			}
		}
		if !cli.outputJoin && !cli.outputYAML {
			if cli.outputNul {
				outStream.Write([]byte{'\x00'})
			} else {
				outStream.Write([]byte{'\n'})
			}
		}
	}
	return nil
}

func (cli *cli) createMarshaler() marshaler {
	if cli.outputYAML {
		return yamlFormatter(cli.outputIndent)
	}
	indent := 2
	if cli.outputCompact {
		indent = 0
	} else if cli.outputTab {
		indent = 1
	} else if i := cli.outputIndent; i != nil {
		indent = *i
	}
	f := newEncoder(cli.outputTab, indent)
	if cli.outputRaw || cli.outputJoin || cli.outputNul {
		return &rawMarshaler{f}
	}
	return f
}

func (cli *cli) printError(err error) {
	if er, ok := err.(interface{ IsEmptyError() bool }); !ok || !er.IsEmptyError() {
		fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
	}
}

// isTTY attempts to determine whether an output is a TTY.
func isTTY(w io.Writer) bool {
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	f, ok := w.(interface{ Fd() uintptr })
	return ok && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()))
}
