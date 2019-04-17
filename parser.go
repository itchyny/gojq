package gojq

import (
	"github.com/alecthomas/participle"
	"github.com/alecthomas/participle/lexer"
	"github.com/alecthomas/participle/lexer/ebnf"
)

var parser = participle.MustBuild(
	&Program{},
	participle.Lexer(lexer.Must(ebnf.New(`
				Ident = ( "_" | alpha ) { "_" | alpha | digit } .
				Recurse = ".." .
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
)

// Parse parses a program.
func Parse(src string) (*Program, error) {
	var program Program
	if err := parser.ParseString(src, &program); err != nil {
		return nil, err
	}
	return &program, nil
}
