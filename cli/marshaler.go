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

func yamlFormatter(indent *int, opts ...cliOption) *yamlMarshaler {
	var c cliConfig
	for _, opt := range opts {
		opt(&c)
	}
	return &yamlMarshaler{indent, c.keys}
}

type yamlMarshaler struct {
	indent *int
	keys   map[uintptr][]string
}

func (m *yamlMarshaler) marshal(v any, w io.Writer) error {
	if len(m.keys) != 0 {
		v = wrap(v, m.keys)
	}
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
