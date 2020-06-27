package gojq

import (
	"fmt"
	"strconv"
)

type lexer struct {
	source    []byte
	offset    int
	result    *Query
	token     string
	tokenType int
	err       error
}

func newLexer(src string) *lexer {
	return &lexer{source: []byte(src)}
}

const eof = -1

func (l *lexer) Lex(lval *yySymType) (tokenType int) {
	defer func() { l.tokenType = tokenType }()
	if len(l.source) == l.offset {
		return eof
	}
	ch := l.next()
	if isIdent(ch, false) {
		l.token = string(l.source[l.offset-1 : l.scanIdent()])
		lval.token = l.token
		return tokIdent
	}
	switch ch {
	case '.':
		if l.peek() == '.' {
			l.offset++
			l.token = ".."
			return tokRecurse
		}
		return '.'
	default:
		return int(ch)
	}
}

func (l *lexer) next() byte {
	for {
		ch := l.source[l.offset]
		l.offset++
		if !isWhite(ch) {
			return ch
		}
	}
}

func (l *lexer) peek() byte {
	if len(l.source) == l.offset {
		return 0
	}
	return l.source[l.offset]
}

func (l *lexer) scanIdent() int {
	for isIdent(l.peek(), true) {
		l.offset++
	}
	return l.offset
}

type parseError struct {
	offset    int
	token     string
	tokenType int
}

func (err *parseError) Error() string {
	var message string
	switch err.tokenType {
	case eof:
		message = "<EOF>"
	case tokIdent, tokRecurse:
		message = strconv.Quote(err.token)
	default:
		message = fmt.Sprintf(`"%c"`, err.tokenType)
	}
	return fmt.Sprintf("unexpected token:%d:%s", err.offset, message)
}

func (l *lexer) Error(e string) {
	l.err = &parseError{l.offset, l.token, l.tokenType}
}

func isWhite(ch byte) bool {
	switch ch {
	case '\t', '\n', '\r', ' ':
		return true
	default:
		return false
	}
}

func isIdent(ch byte, tail bool) bool {
	return 'a' <= ch && ch <= 'z' || ch == '_' || tail && isNumber(ch)
}

func isNumber(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
