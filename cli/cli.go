// Package cli implements the gojq command.
package cli

import (
	"errors"
	"fmt"
	"github.com/itchyny/gojq"
	"github.com/mattn/go-isatty"
	"io"
	"os"
	"runtime"
	"strings"
)

const name = "gojq"

const version = "0.12.15"

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

	outputRaw     bool
	outputRaw0    bool
	outputJoin    bool
	outputCompact bool
	outputIndent  *int
	outputTab     bool
	outputJSON    bool
	outputXML     bool
	outputYAML    bool
	inputRaw      bool
	inputStream   bool
	inputJSON     bool
	inputXML      bool
	inputYAML     bool
	inputSlurp    bool
	stripSpaceXML bool
	stripAttrsXML bool
	forceListXML  []string
	htmlXML       bool
	rootXML       string
	elementXML    string

	argnames  []string
	argvalues []any

	outputYAMLSeparator bool
	exitCodeError       error
}

type flagopts struct {
	OutputRaw     bool              `short:"r" long:"raw-output" description:"output raw strings"`
	OutputRaw0    bool              `long:"raw-output0" description:"implies -r with NUL character delimiter"`
	OutputJoin    bool              `short:"j" long:"join-output" description:"implies -r with no newline delimiter"`
	OutputCompact bool              `short:"c" long:"compact-output" description:"output without pretty-printing"`
	OutputIndent  *int              `long:"indent" description:"number of spaces for indentation"`
	OutputTab     bool              `long:"tab" description:"use tabs for indentation"`
	OutputYAML    bool              `short:"y" long:"yaml-output" description:"output in YAML format"`
	OutputXML     bool              `short:"x" long:"xml-output" description:"output in XML format"`
	OutputColor   bool              `short:"C" long:"color-output" description:"output with colors even if piped"`
	OutputMono    bool              `short:"M" long:"monochrome-output" description:"output without colors"`
	InputNull     bool              `short:"n" long:"null-input" description:"use null as input value"`
	InputRaw      bool              `short:"R" long:"raw-input" description:"read input as raw strings"`
	InputStream   bool              `long:"stream" description:"parse input in stream fashion"`
	InputJSON     bool              `short:"J" long:"json-input" description:"read input as JSON format"`
	InputXML      bool              `short:"X" long:"xml-input" description:"read input as XML format"`
	StripAttrsXML bool              `long:"xml-no-attributes" description:"remove attributes from XML elements"`
	StripSpaceXML bool              `long:"xml-no-namespaces" description:"remove namespace from XML elements and attributes"`
	ForceListXML  []string          `long:"xml-force-list" description:"force XML elements as array"`
	RootXML       string            `long:"xml-root" description:"root XML element name"`
	ElementXML    string            `long:"xml-element" description:"element XML element name"`
	HtmlXML       bool              `short:"H" long:"xml-html" description:"read input as XML with HTML compatibility mode"`
	InputYAML     bool              `short:"Y" long:"yaml-input" description:"read input as YAML format"`
	InputSlurp    bool              `short:"s" long:"slurp" description:"read all inputs into an array"`
	FromFile      bool              `short:"f" long:"from-file" description:"load query from file"`
	ModulePaths   []string          `short:"L" description:"directory to search modules from"`
	Arg           map[string]string `long:"arg" description:"set a string value to a variable"`
	ArgJSON       map[string]string `long:"argjson" description:"set a JSON value to a variable"`
	SlurpFile     map[string]string `long:"slurpfile" description:"set the JSON contents of a file to a variable"`
	RawFile       map[string]string `long:"rawfile" description:"set the contents of a file to a variable"`
	Args          []any             `long:"args" positional:"" description:"consume remaining arguments as positional string values"`
	JSONArgs      []any             `long:"jsonargs" positional:"" description:"consume remaining arguments as positional JSON values"`
	ExitStatus    bool              `short:"e" long:"exit-status" description:"exit 1 when the last value is false or null"`
	Version       bool              `short:"v" long:"version" description:"display version information"`
	Help          bool              `short:"h" long:"help" description:"display this help information"`
}

var addDefaultModulePaths = true

func (cli *cli) run(args []string) int {
	if err := cli.runInternal(args); err != nil {
		if _, ok := err.(interface{ isEmptyError() }); !ok {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, err)
		}
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
	cli.outputRaw, cli.outputRaw0, cli.outputJoin,
		cli.outputCompact, cli.outputIndent, cli.outputTab, cli.outputXML, cli.outputYAML =
		opts.OutputRaw, opts.OutputRaw0, opts.OutputJoin,
		opts.OutputCompact, opts.OutputIndent, opts.OutputTab, opts.OutputXML, opts.OutputYAML
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
	cli.inputRaw, cli.inputStream, cli.inputYAML, cli.inputSlurp =
		opts.InputRaw, opts.InputStream, opts.InputYAML, opts.InputSlurp
	cli.inputJSON, cli.inputXML, cli.stripAttrsXML, cli.stripSpaceXML, cli.forceListXML, cli.rootXML, cli.elementXML, cli.htmlXML =
		opts.InputJSON, opts.InputXML, opts.StripAttrsXML, opts.StripSpaceXML, opts.ForceListXML, opts.RootXML, opts.ElementXML, opts.HtmlXML
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
	named := make(map[string]any, len(cli.argnames))
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
	cli.argvalues = append(cli.argvalues, map[string]any{
		"named":      named,
		"positional": positional,
	})
	var arg, fname string
	if opts.FromFile {
		if len(args) == 0 {
			return errors.New("expected a query file for flag `-f'")
		}
		src, err := os.ReadFile(args[0])
		if err != nil {
			return err
		}
		arg, args, fname = string(src), args[1:], args[0]
	} else if len(args) == 0 {
		arg = "."
	} else {
		arg, args, fname = strings.TrimSpace(args[0]), args[1:], "<arg>"
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
		modulePaths = []string{"~/.jq", "$ORIGIN/../lib/gojq", "$ORIGIN/../lib"}
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
			func(iter inputIter) func(any, []any) any {
				return func(any, []any) any {
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

func slurpFile(name string) (any, error) {
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
	case cli.inputJSON:
		newIter = newJSONInputIter
	case cli.inputXML || cli.htmlXML:
		newIter = func(r io.Reader, fname string) inputIter {
			return newXMLInputIter(r, fname, !cli.stripAttrsXML, !cli.stripSpaceXML, cli.forceListXML, cli.htmlXML)
		}
	case cli.inputYAML:
		newIter = newYAMLInputIter
	default:
		// automatically detect between JSON / YAML / XML format
		newIter = func(r io.Reader, fname string) inputIter {
			rd, f := detectInputType(r, 100)
			switch f {
			case JsonFormat:
				return newJSONInputIter(rd, fname)
			case YamlFormat:
				return newYAMLInputIter(rd, fname)
			case XmlFormat:
				return newXMLInputIter(rd, fname, !cli.stripAttrsXML, !cli.stripSpaceXML, cli.forceListXML, cli.htmlXML)
			}
			return newJSONInputIter(rd, fname)
		}
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
			break
		}
		if e, ok := v.(error); ok {
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, e)
			err = e
			continue
		}
		if e := cli.printValues(code.Run(v, cli.argvalues...)); e != nil {
			if e, ok := e.(*gojq.HaltError); ok {
				if v := e.Value(); v != nil {
					if str, ok := v.(string); ok {
						cli.errStream.Write([]byte(str))
					} else {
						bs, _ := gojq.Marshal(v)
						cli.errStream.Write(bs)
						cli.errStream.Write([]byte{'\n'})
					}
				}
				err = e
				break
			}
			fmt.Fprintf(cli.errStream, "%s: %s\n", name, e)
			err = e
		}
	}
	if err != nil {
		return &emptyError{err}
	}
	return nil
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
		if !cli.outputYAML {
			if cli.outputRaw0 {
				cli.outStream.Write([]byte{'\x00'})
			} else if !cli.outputJoin {
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
	if cli.outputXML {
		return xmlFormatter(&indent, cli.rootXML, cli.elementXML)
	}
	f := newEncoder(cli.outputTab, indent)
	if cli.outputRaw || cli.outputRaw0 || cli.outputJoin {
		return &rawMarshaler{f, cli.outputRaw0}
	}
	return f
}

func (cli *cli) funcDebug(v any, _ []any) any {
	if err := newEncoder(false, 0).
		marshal([]any{"DEBUG:", v}, cli.errStream); err != nil {
		return err
	}
	if _, err := cli.errStream.Write([]byte{'\n'}); err != nil {
		return err
	}
	return v
}

func (cli *cli) funcStderr(v any, _ []any) any {
	if err := (&rawMarshaler{m: newEncoder(false, 0)}).
		marshal(v, cli.errStream); err != nil {
		return err
	}
	return v
}
