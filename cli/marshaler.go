package cli

import (
	"fmt"
	"github.com/momiji/xqml"
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

func xmlFormatter(indent *int, root string, element string) *xmlMarshaller {
	return &xmlMarshaller{indent, root, element}
}

type xmlMarshaller struct {
	indent  *int
	root    string
	element string
}

func (m *xmlMarshaller) marshal(v any, w io.Writer) error {
	enc := xqml.NewEncoder(w)
	if i := m.indent; i != nil {
		indent := strings.Repeat(" ", *m.indent)
		enc.Indent = indent
	}
	if m.root != "" {
		enc.Root = m.root
	}
	if m.element != "" {
		enc.Element = m.element
	}
	return enc.Encode(v)
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
