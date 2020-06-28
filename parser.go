package gojq

//go:generate go run _tools/gen_string.go -o string.go

const numberPatternStr = `(?:\d*\.)?\d+(?:[eE][-+]?\d+)?\b`

// Parse parses a query.
func Parse(src string) (*Query, error) {
	m, err := parse(src)
	if err != nil {
		return nil, err
	}
	m.Query.Imports = m.Imports
	m.Query.Commas[0].Filters[0].FuncDefs = m.FuncDefs
	return m.Query, nil
}

// ParseModule parses a module.
func ParseModule(src string) (*Module, error) {
	return parse(src)
}

func parse(src string) (*Module, error) {
	l := newLexer(src)
	if yyParse(l) > 0 {
		return nil, l.err
	}
	return l.result, nil
}
