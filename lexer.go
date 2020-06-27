package gojq

import (
	"fmt"
	"strconv"
	"unicode/utf8"
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

var keywords = map[string]int{
	"if":    tokIf,
	"then":  tokThen,
	"elif":  tokElif,
	"else":  tokElse,
	"end":   tokEnd,
	"try":   tokTry,
	"catch": tokCatch,
}

func (l *lexer) Lex(lval *yySymType) (tokenType int) {
	defer func() { l.tokenType = tokenType }()
	if len(l.source) == l.offset {
		return eof
	}
	ch := l.next()
	switch {
	case isIdent(ch, false):
		l.token = string(l.source[l.offset-1 : l.scanIdent()])
		lval.token = l.token
		if tok, ok := keywords[l.token]; ok {
			return tok
		}
		return tokIdent
	case isNumber(ch):
		i := l.offset - 1
		j := l.scanNumber(numberStateLead)
		if j < 0 {
			l.token = string(l.source[i:-j])
			return tokInvalid
		}
		l.token = string(l.source[i:j])
		lval.token = l.token
		return tokNumber
	}
	switch ch {
	case '.':
		ch := l.peek()
		switch {
		case ch == '.':
			l.offset++
			l.token = ".."
			return tokRecurse
		case isIdent(ch, false):
			l.token = string(l.source[l.offset-1 : l.scanIdent()])
			lval.token = l.token
			return tokIndex
		case isNumber(ch):
			i := l.offset - 1
			j := l.scanNumber(numberStateFloat)
			if j < 0 {
				l.token = string(l.source[i:-j])
				return tokInvalid
			}
			l.token = string(l.source[i:j])
			lval.token = l.token
			return tokNumber
		default:
			return '.'
		}
	case '$':
		if !isIdent(l.peek(), false) {
			return int(ch)
		}
		l.token = string(l.source[l.offset-1 : l.scanIdent()])
		lval.token = l.token
		return tokVariable
	default:
		if ch < utf8.RuneSelf {
			return int(ch)
		}
		r, _ := utf8.DecodeRune(l.source[l.offset-1:])
		l.token = string(r)
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

const (
	numberStateLead = iota
	numberStateFloat
	numberStateExpLead
	numberStateExp
)

func (l *lexer) scanNumber(state int) int {
	for {
		switch state {
		case numberStateLead, numberStateFloat:
			if ch := l.peek(); isNumber(ch) {
				l.offset++
			} else {
				switch ch {
				case '.':
					if state != numberStateLead {
						return l.offset
					}
					l.offset++
					state = numberStateFloat
				case 'e', 'E':
					l.offset++
					switch l.peek() {
					case '-', '+':
						l.offset++
					}
					state = numberStateExpLead
				default:
					if isIdent(ch, false) {
						l.offset++
						return -l.offset
					}
					return l.offset
				}
			}
		case numberStateExpLead, numberStateExp:
			if ch := l.peek(); !isNumber(ch) {
				if isIdent(ch, false) {
					l.offset++
					return -l.offset
				}
				if state == numberStateExpLead && len(l.source) == l.offset {
					return -l.offset
				}
				return l.offset
			}
			l.offset++
			state = numberStateExp
		default:
			panic(state)
		}
	}
}

type parseError struct {
	offset    int
	token     string
	tokenType int
}

func (err *parseError) Error() string {
	var message string
	switch {
	case err.tokenType == eof:
		message = "<EOF>"
	case err.tokenType >= utf8.RuneSelf:
		message = strconv.Quote(err.token)
	default:
		message = fmt.Sprintf(`"%c"`, rune(err.tokenType))
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
	return 'a' <= ch && ch <= 'z' ||
		'A' <= ch && ch <= 'Z' || ch == '_' ||
		tail && isNumber(ch)
}

func isNumber(ch byte) bool {
	return '0' <= ch && ch <= '9'
}
