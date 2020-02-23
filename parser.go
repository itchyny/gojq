package gojq

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
)

var parserOptions = []participle.Option{
	participle.Lexer(lexer.Must(lexer.Regexp(`(\s+|#[^\n]*)` +
		`|(?P<Keyword>(import|include|null|true|false|if|then|elif|else|end|or|and|as|try|catch|reduce|foreach|label|break)\b)` +
		`|(?P<ModuleIdent>\$?[a-zA-Z_][a-zA-Z0-9_]*::[a-zA-Z_][a-zA-Z0-9_]*)` +
		`|(?P<Ident>\$?[a-zA-Z_][a-zA-Z0-9_]*)` +
		`|(?P<UpdateAltOp>(//=))` +
		`|(?P<Op>(\.\.|\??//))` +
		`|(?P<CompareOp>([=!]=|[<>]=?))` +
		`|(?P<UpdateOp>(=|[-|+*/%]=))` +
		`|(?P<Number>((\d*\.)?\d+([eE]([-+]?\d+))?\b))` +
		`|(?P<String>"([^"\\]*|\\.)*")` +
		`|(?P<Format>@[a-zA-Z0-9_]+)` +
		"|(?P<Punct>[!-/:-@\\[-\\]^-`{-~])",
	))),
	participle.UseLookahead(2),
}

var parser = participle.MustBuild(&Query{}, parserOptions...)

// Parse parses a query.
func Parse(src string) (*Query, error) {
	var query Query
	if err := parser.ParseString(src, &query); err != nil {
		return nil, err
	}
	return &query, nil
}

var modulesParser = participle.MustBuild(&Module{}, parserOptions...)

// ParseModule parses a module.
func ParseModule(src string) (*Module, error) {
	var module Module
	if err := modulesParser.ParseString(src, &module); err != nil {
		return nil, err
	}
	return &module, nil
}
