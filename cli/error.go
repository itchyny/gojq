package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/alecthomas/participle"
	"github.com/mattn/go-runewidth"
)

type errors []error

func (errs errors) Error() string {
	var s strings.Builder
	for i, err := range errs {
		if i > 0 {
			fmt.Fprint(&s, ", ")
		}
		fmt.Fprint(&s, err)
	}
	return s.String()
}

type variableNameError struct {
	name string
}

func (err *variableNameError) Error() string {
	return "invalid variable name: " + err.name
}

type compileError struct {
	err error
}

func (err *compileError) Error() string {
	return "compile error: " + err.err.Error()
}

type queryParseError struct {
	fname, contents string
	err             error
}

func (err *queryParseError) Error() string {
	var s strings.Builder
	if er, ok := err.err.(participle.Error); ok {
		lines := strings.Split(err.contents, "\n")
		if 0 < er.Position().Line && er.Position().Line <= len(lines) {
			var prefix, fname string
			if len(lines) <= 1 && err.fname == "<arg>" {
				fname = err.contents
			} else {
				fname = fmt.Sprintf("%s:%d", err.fname, er.Position().Line)
				prefix = fmt.Sprintf("%d | ", er.Position().Line)
			}
			fmt.Fprintf(&s, "invalid query: %s\n", fname)
			fmt.Fprintf(
				&s, "    %s%s\n%s  %s\n", prefix, lines[er.Position().Line-1],
				strings.Repeat(" ", 3+er.Position().Column+len(prefix))+"^", er.Message())
			return s.String()
		}
	}
	fmt.Fprintf(&s, "invalid query: %s: %s\n", err.fname, err.err)
	return s.String()
}

type jsonParseError struct {
	fname, contents string
	err             error
}

func (err *jsonParseError) Error() string {
	var s strings.Builder
	fmt.Fprintf(&s, "invalid json: %s\n", err.fname)
	if er := err.err; er.Error() == "unexpected EOF" {
		lines := strings.Split(strings.TrimRight(err.contents, "\n"), "\n")
		line := toValidUTF8(strings.TrimRight(lines[len(lines)-1], "\r"))
		fmt.Fprintf(&s, "    %s\n%s  %s\n", line, strings.Repeat(" ", 4+runewidth.StringWidth(line))+"^", er)
	} else if er, ok := er.(*json.SyntaxError); ok {
		var ss strings.Builder
		var i, j int
		for _, r := range toValidUTF8(err.contents) {
			i += len([]byte(string(r)))
			if i <= int(er.Offset) {
				j += runewidth.RuneWidth(r)
			}
			if r == '\n' || r == '\r' {
				if i == int(er.Offset) {
					j++
					break
				} else if i > int(er.Offset) {
					break
				} else {
					j = 0
					ss.Reset()
				}
			} else {
				ss.WriteRune(r)
			}
		}
		fmt.Fprintf(&s, "    %s\n%s  %s\n", ss.String(), strings.Repeat(" ", 3+j)+"^", er)
	}
	return s.String()
}

type yamlParseError struct {
	fname, contents string
	err             error
}

func (err *yamlParseError) Error() string {
	var s strings.Builder
	fmt.Fprintf(&s, "invalid yaml: %s\n", err.fname)
	var line int
	msg := err.err.Error()
	fmt.Fscanf(strings.NewReader(msg), "yaml: line %d:", &line)
	if line == 0 {
		return s.String()
	}
	msg = msg[7+strings.IndexRune(msg[5:], ':'):]
	var ss strings.Builder
	var i, j int
	var cr bool
	for _, r := range toValidUTF8(err.contents) {
		i += len([]byte(string(r)))
		if r == '\n' || r == '\r' {
			if !cr || r != '\n' {
				j++
			}
			cr = r == '\r'
			if j == line {
				break
			}
			ss.Reset()
		} else {
			cr = false
			ss.WriteRune(r)
		}
	}
	fmt.Fprintf(&s, "    %s\n    ^  %s\n", ss.String(), msg)
	return s.String()
}

func toValidUTF8(s string) string {
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
