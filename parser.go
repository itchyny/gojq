package gojq

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/participle/lexer/ebnf"
)

// Parser parses a query.
type Parser interface {
	Parse(string) (*Query, error)
}

// NewParser creates a new query parser.
func NewParser() Parser {
	return &parser{
		participle.MustBuild(
			&Query{},
			participle.Lexer(lexer.Must(ebnf.New(`
				ObjectIndex = "." { "_" | alpha } { "_" | alpha | digit } .
				alpha = "a"…"z" | "A"…"Z" .
				digit = "0"…"9" .
`))),
		),
	}
}

type parser struct {
	*participle.Parser
}

func (p *parser) Parse(s string) (*Query, error) {
	var query Query
	if err := p.ParseString(s, &query); err != nil {
		return nil, err
	}
	return &query, nil
}
