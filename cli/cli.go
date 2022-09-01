// Package cli implements the gojq command.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mattn/go-isatty"

	"github.com/itchyny/gojq"
)

const name = "gojq"

const version = "0.12.9"

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
	Arg           map[string]string `long:"arg" description:"set variable to string value"`
	ArgJSON       map[string]string `long:"argjson" description:"set variable to JSON value"`
	SlurpFile     map[string]string `long:"slurpfile" description:"set variable to the JSON contents of the file"`
	RawFile       map[string]string `long:"rawfile" description:"set variable to the contents of the file"`
	Args          []interface{}     `long:"args" positional:"" description:"consume remaining arguments as positional string values"`
	JSONArgs      []interface{}     `long:"jsonargs" positional:"" description:"consume remaining arguments as positional JSON values"`
	ExitStatus    bool              `short:"e" long:"exit-status" description:"exit 1 when the last value is false or null"`
	Version       bool              `short:"v" long:"version" description:"print version"`
	Help          bool              `short:"h" long:"help" description:"print this help"`
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
	args, err = parseFlags(args, &opts)
	if err != nil {
		return &flagParseError{err}
	}
	if opts.Help {
		fmt.Fprintf(cli.outStream, `%[1]s - Go implementation of jq

Version: %s (rev: %s/%s)

Synopsis:
  %% echo '{"foo": 128}' | %[1]s '.foo'

Usage:
  %[1]s [OPTIONS]

`,
			name, version, revision, runtime.Version())
		fmt.Fprintln(cli.outStream, formatFlags(&opts))
		return nil
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
	} else if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		noColor = true
	} else {
		f, ok := cli.outStream.(interface{ Fd() uintptr })
		noColor = !(ok && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd())))
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
	for k, v := range opts.Arg {
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, v)
	}
	for k, v := range opts.ArgJSON {
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
		val, err := os.ReadFile(v)
		if err != nil {
			return err
		}
		cli.argnames = append(cli.argnames, "$"+k)
		cli.argvalues = append(cli.argvalues, string(val))
	}
	named := make(map[string]interface{}, len(cli.argnames))
	for i, name := range cli.argnames {
		named[name[1:]] = cli.argvalues[i]
	}
	positional := opts.Args
	for i, v := range opts.JSONArgs {
		if v != nil {
			val, _ := newJSONInputIter(strings.NewReader(v.(string)), "--jsonargs").Next()
			if err, ok := val.(error); ok {
				return err
			}
			if i < len(positional) {
				positional[i] = val
			} else {
				positional = append(positional, val)
			}
		}
	}
	cli.argnames = append(cli.argnames, "$ARGS")
	cli.argvalues = append(cli.argvalues, map[string]interface{}{
		"named":      named,
		"positional": positional,
	})
	var arg, fname string
	if opts.FromFile != "" {
		src, err := os.ReadFile(opts.FromFile)
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
			if _, ok := err.(interface{ ExitCode() int }); !ok {
				err = cli.exitCodeError
			}
		}()
	}
	query, err := gojq.Parse(arg)
	if err != nil {
		return &queryParseError{fname, arg, err}
	}
	modulePaths := opts.ModulePaths
	if len(modulePaths) == 0 && addDefaultModulePaths {
		modulePaths = listDefaultModulePaths()
	}
	iter := cli.createInputIter(args)
	defer iter.Close()
	code, err := gojq.Compile(query,
		gojq.WithModuleLoader(gojq.NewModuleLoader(modulePaths)),
		gojq.WithEnvironLoader(os.Environ),
		gojq.WithVariables(cli.argnames),
		gojq.WithFunction("debug", 0, 0, cli.funcDebug),
		gojq.WithFunction("stderr", 0, 0, cli.funcStderr),
		gojq.WithFunction("input_filename", 0, 0,
			func(iter inputIter) func(interface{}, []interface{}) interface{} {
				return func(interface{}, []interface{}) interface{} {
					if fname := iter.Name(); fname != "" && (len(args) > 0 || !opts.InputNull) {
						return fname
					}
					return nil
				}
			}(iter),
		),
		gojq.WithInputIter(iter),
	)
	if err != nil {
		if err, ok := err.(interface {
			QueryParseError() (string, string, error)
		}); ok {
			name, query, err := err.QueryParseError()
			return &queryParseError{name, query, err}
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

func listDefaultModulePaths() []string {
	modulePaths := []string{"", "../lib/gojq", "lib"}
	if executable, err := os.Executable(); err == nil {
		if executable, err := filepath.EvalSymlinks(executable); err == nil {
			origin := filepath.Dir(executable)
			modulePaths[1] = filepath.Join(origin, modulePaths[1])
			modulePaths[2] = filepath.Join(origin, modulePaths[2])
		}
	}
	if homeDir, err := os.UserHomeDir(); err == nil {
		modulePaths[0] = filepath.Join(homeDir, ".jq")
	} else {
		modulePaths = modulePaths[1:]
	}
	return modulePaths
}

func slurpFile(name string) (interface{}, error) {
	iter := newSlurpInputIter(
		newFilesInputIter(newJSONInputIter, []string{name}, nil),
	)
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
		if cli.inputSlurp {
			newIter = newReadAllIter
		} else {
			newIter = newRawInputIter
		}
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
	return newFilesInputIter(newIter, args, cli.inStream)
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

func (cli *cli) printValues(iter gojq.Iter) error {
	m := cli.createMarshaler()
	for {
		v, ok := iter.Next()
		if !ok {
			break
		}
		if err, ok := v.(error); ok {
			return err
		}
		if cli.outputYAMLSeparator {
			cli.outStream.Write([]byte("---\n"))
		} else {
			cli.outputYAMLSeparator = cli.outputYAML
		}
		if err := m.marshal(v, cli.outStream); err != nil {
			return err
		}
		if cli.exitCodeError != nil {
			if v == nil || v == false {
				cli.exitCodeError = &exitCodeError{exitCodeFalsyErr}
			} else {
				cli.exitCodeError = &exitCodeError{exitCodeOK}
			}
		}
		if !cli.outputJoin && !cli.outputYAML {
			if cli.outputNul {
				cli.outStream.Write([]byte{'\x00'})
			} else {
				cli.outStream.Write([]byte{'\n'})
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

func (cli *cli) funcDebug(v interface{}, _ []interface{}) interface{} {
	newEncoder(false, 0).marshal([]interface{}{"DEBUG:", v}, cli.errStream)
	cli.errStream.Write([]byte{'\n'})
	return v
}

func (cli *cli) funcStderr(v interface{}, _ []interface{}) interface{} {
	newEncoder(false, 0).marshal(v, cli.errStream)
	return v
}

func (cli *cli) printError(err error) {
	if er, ok := err.(interface{ IsEmptyError() bool }); !ok || !er.IsEmptyError() {
		if er, ok := err.(interface{ IsHaltError() bool }); !ok || !er.IsHaltError() {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		} else if er, ok := err.(gojq.ValueError); ok {
			v := er.Value()
			if str, ok := v.(string); ok {
				cli.errStream.Write([]byte(str))
			} else {
				bs, _ := gojq.Marshal(v)
				cli.errStream.Write(bs)
				cli.errStream.Write([]byte{'\n'})
			}
		}
	}
}
