package cli

import (
	"bytes"

	"github.com/fatih/color"
	"github.com/hokaccha/go-prettyjson"
	"gopkg.in/yaml.v3"
)

type marshaler interface {
	Marshal(v interface{}) ([]byte, error)
}

func jsonFormatter() *prettyjson.Formatter {
	f := prettyjson.NewFormatter()
	f.StringColor = color.New(color.FgGreen)
	f.BoolColor = color.New(color.FgYellow)
	f.NumberColor = color.New(color.FgCyan)
	f.NullColor = color.New(color.FgHiBlack)
	return f
}

type rawMarshaler struct {
	m marshaler
}

func (m *rawMarshaler) Marshal(v interface{}) ([]byte, error) {
	if s, ok := v.(string); ok {
		return []byte(s), nil
	}
	return m.m.Marshal(v)
}

func yamlFormatter() *yamlMarshaler {
	return &yamlMarshaler{}
}

type yamlMarshaler struct{}

func (m *yamlMarshaler) Marshal(v interface{}) ([]byte, error) {
	var bs bytes.Buffer
	enc := yaml.NewEncoder(&bs)
	enc.SetIndent(2)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return bs.Bytes(), nil
}
