package cli

import (
	"io"

	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"gopkg.in/yaml.v3"
)

type marshaler interface {
	marshal(interface{}, io.Writer) error
}

func jsonFormatter(indent int, compact bool) *jsonMarshaler {
	f := prettyjson.NewFormatter()
	if compact {
		f.Indent, f.Newline = 0, ""
	} else {
		f.Indent = indent
	}
	f.StringColor = color.New(color.FgGreen)
	f.BoolColor = color.New(color.FgYellow)
	f.NumberColor = color.New(color.FgCyan)
	f.NullColor = color.New(color.FgHiBlack)
	return &jsonMarshaler{f}
}

type jsonMarshaler struct {
	f *prettyjson.Formatter
}

func (m *jsonMarshaler) marshal(v interface{}, w io.Writer) error {
	xs, err := m.f.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(xs)
	return err
}

type rawMarshaler struct {
	m marshaler
}

func (m *rawMarshaler) marshal(v interface{}, w io.Writer) error {
	if s, ok := v.(string); ok {
		_, err := w.Write([]byte(s))
		return err
	}
	return m.m.marshal(v, w)
}

func yamlFormatter(indent *int) *yamlMarshaler {
	return &yamlMarshaler{indent}
}

type yamlMarshaler struct {
	indent *int
}

func (m *yamlMarshaler) marshal(v interface{}, w io.Writer) error {
	enc := yaml.NewEncoder(w)
	if i := m.indent; i != nil {
		enc.SetIndent(*i)
	} else {
		enc.SetIndent(2)
	}
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}
