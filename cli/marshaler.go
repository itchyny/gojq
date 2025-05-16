package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/goccy/go-yaml"
)

type marshaler interface {
	marshal(any, io.Writer) error
}

type rawMarshaler struct {
	m        marshaler
	checkNul bool
}

func (m *rawMarshaler) marshal(v any, w io.Writer) error {
	if s, ok := v.(string); ok {
		if m.checkNul && strings.ContainsRune(s, '\x00') {
			return fmt.Errorf("cannot output a string containing NUL character: %q", s)
		}
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

func (m *yamlMarshaler) marshal(v any, w io.Writer) error {
	indent := 2
	if i := m.indent; i != nil {
		indent = *i
	}
	_, isArray := v.([]any) // https://github.com/goccy/go-yaml/issues/291
	enc := yaml.NewEncoder(w, yaml.Indent(indent), yaml.IndentSequence(!isArray))
	if err := enc.Encode(v); err != nil {
		return err
	}
	return enc.Close()
}
