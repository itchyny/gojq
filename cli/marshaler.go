package cli

import (
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
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
