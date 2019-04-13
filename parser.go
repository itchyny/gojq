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
				Ident = ( "_" | alpha ) { "_" | alpha | digit } .
				Integer = "0" | "1"…"9" { digit } .
				String = "\""  { "\u0000"…"\uffff"-"\""-"\\" | "\\" any } "\"" .
				Whitespace = " " | "\t" | "\n" | "\r" .
				Punct = "!"…"/" | ":"…"@" | "["…`+"\"`\""+` | "{"…"~" .
				alpha = "a"…"z" | "A"…"Z" .
				digit = "0"…"9" .
				any = "\u0000"…"\uffff" .
`))),
			participle.Elide("Whitespace"),
			participle.Unquote("String"),
			participle.UseLookahead(2),
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
