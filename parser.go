package gojq

import "github.com/alecthomas/participle"

// Parser parses a query.
type Parser interface {
	Parse(string) (*Query, error)
}

// NewParser creates a new query parser.
func NewParser() Parser {
	return &parser{participle.MustBuild(&Query{})}
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
