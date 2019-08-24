package gojq

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
)

var parser = participle.MustBuild(
	&Query{},
	participle.Lexer(lexer.Must(lexer.Regexp(`(\s+|#[^\n]*)`+
		`|(?P<Keyword>(null|true|false|if|then|elif|else|end|or|and|as|try|catch|reduce|foreach|label|break)\b)`+
		`|(?P<Ident>\$?[a-zA-Z_][a-zA-Z0-9_]*)`+
		`|(?P<UpdateAltOp>(//=))`+
		`|(?P<Op>(\.\.|\??//))`+
		`|(?P<CompareOp>([=!]=|[<>]=?))`+
		`|(?P<UpdateOp>(=|[-|+*/%]=))`+
		`|(?P<Number>((\d*\.)?\d+([eE]([-+]?\d+))?\b))`+
		`|(?P<String>"([^"\\]*|\\.)*")`+
		"|(?P<Punct>[!-/:-@\\[-\\]^-`{-~])",
	))),
	participle.UseLookahead(2),
)

// Parse parses a query.
func Parse(src string) (*Query, error) {
	var query Query
	if err := parser.ParseString(src, &query); err != nil {
		return nil, err
	}
	return &query, nil
}
