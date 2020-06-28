package gojq

//go:generate go run _tools/gen_string.go -o string.go

// Parse parses a query.
func Parse(src string) (*Query, error) {
	l := newLexer(src)
	if yyParse(l) > 0 {
		return nil, l.err
	}
	return l.result, nil
}
