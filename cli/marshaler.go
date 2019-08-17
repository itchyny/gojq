package cli

import (
	"github.com/fatih/color"
	"github.com/itchyny/go-prettyjson"
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
