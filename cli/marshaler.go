package cli

import (
	"github.com/clbanning/mxj/v2"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type marshaler interface {
	marshal(any, io.Writer) error
}

type rawMarshaler struct {
	m marshaler
}

func (m *rawMarshaler) marshal(v any, w io.Writer) error {
	if s, ok := v.(string); ok {
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
	indentStr := "  "
	if i := m.indent; i != nil {
		indentStr = strings.Repeat(" ", *m.indent)
	}
	tags := []string{m.root, m.element}
	if m.root == "" {
		tags[0] = mxj.DefaultRootTag
	}
	if m.element == "" {
		tags[1] = mxj.DefaultElementTag
	}
	bytes, err := mxj.AnyXmlIndent(v, "", indentStr, tags...)
	if err != nil {
		return err
	}
	_, err = w.Write(bytes)
	return err
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
