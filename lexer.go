package gojq

import "fmt"

type lexer struct {
	source []byte
	offset int
	result *Query
	err    error
}

func newLexer(src string) *lexer {
	return &lexer{source: []byte(src)}
}

const eof = -1

func (l *lexer) Lex(lval *yySymType) int {
	if len(l.source) == l.offset {
		return eof
	}
	ch := l.source[l.offset]
	l.offset++
	switch ch {
	case '.':
		if l.peek() == '.' {
			l.offset++
			return tokRecurse
		}
		return '.'
	default:
		return int(ch)
	}
}

func (l *lexer) peek() byte {
	if len(l.source) == l.offset {
		return 0
	}
	return l.source[l.offset]
}

func (l *lexer) Error(e string) {
	l.err = fmt.Errorf("unexpected token")
}
