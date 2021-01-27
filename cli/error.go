package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

type emptyError struct {
	err error
}

func (*emptyError) Error() string {
	return ""
}

func (*emptyError) IsEmptyError() bool {
	return true
}

func (err *emptyError) ExitCode() int {
	if err, ok := err.err.(interface{ ExitCode() int }); ok {
		return err.ExitCode()
	}
	return exitCodeDefaultErr
}

type exitCodeError struct {
	code int
}

func (err *exitCodeError) Error() string {
	return "exit code: " + strconv.Itoa(err.code)
}

func (err *exitCodeError) IsEmptyError() bool {
	return true
}

func (err *exitCodeError) ExitCode() int {
	return err.code
}

type flagParseError struct {
	err error
}

func (err *flagParseError) Error() string {
	return err.err.Error()
}

func (err *flagParseError) ExitCode() int {
	return exitCodeFlagParseErr
}

type compileError struct {
	err error
}

func (err *compileError) Error() string {
	return "compile error: " + err.err.Error()
}

func (err *compileError) ExitCode() int {
	return exitCodeCompileErr
}

type queryParseError struct {
	typ, fname, contents string
	err                  error
}

func (err *queryParseError) Error() string {
	var s strings.Builder
	if er, ok := err.err.(interface{ Token() (string, int) }); ok {
		_, offset := er.Token()
		var ss strings.Builder
		var i, j int
		line, total := 1, len(err.contents)
		for _, r := range toValidUTF8(err.contents) {
			if i+len(string(r)) < offset {
				j += runewidth.RuneWidth(r)
			}
			i += len(string(r))
			if r == '\n' || r == '\r' {
				if i == int(offset) {
					j++
					break
				} else if i > int(offset) {
					break
				} else if i < total {
					j = 0
					if r == '\n' {
						line++
					}
					ss.Reset()
				}
			} else {
				ss.WriteRune(r)
			}
		}
		var prefix, fname string
		if !strings.ContainsAny(err.contents, "\n\r") && strings.HasPrefix(err.fname, "<arg>") {
			fname = err.contents
		} else {
			fname = err.fname + ":" + strconv.Itoa(line)
			prefix = strconv.Itoa(line) + " | "
		}
		fmt.Fprintf(&s, "invalid %s: %s\n", err.typ, fname)
		fmt.Fprintf(
			&s, "    %s%s\n    %s  %s", prefix, ss.String(),
			strings.Repeat(" ", j+len(prefix))+"^", er)
		return s.String()
	}
	fmt.Fprintf(&s, "invalid %s: %s: %s", err.typ, err.fname, err.err)
	return s.String()
}

func (err *queryParseError) ExitCode() int {
	return exitCodeCompileErr
}

type jsonParseError struct {
	fname, contents string
	err             error
}

func (err *jsonParseError) Error() string {
	var s strings.Builder
	fmt.Fprintf(&s, "invalid json: %s", err.fname)
	if er := err.err; er.Error() == "unexpected EOF" {
		lines := strings.Split(strings.TrimRight(err.contents, "\n"), "\n")
		line := toValidUTF8(strings.TrimRight(lines[len(lines)-1], "\r"))
		fmt.Fprintf(&s, "\n    %s\n%s  %s", line, strings.Repeat(" ", 4+runewidth.StringWidth(line))+"^", er)
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
		rs := []rune(ss.String())
		for len(rs) > 100 {
			k := len(rs) / 2
			l := runewidth.StringWidth(string(rs[:k]))
			if j < l+10 {
				k /= 2
				l = runewidth.StringWidth(string(rs[:k]))
				if j < l+10 {
					rs = rs[:k*2]
				} else {
					j -= l
					rs = rs[k:]
				}
			} else {
				j -= l
				rs = rs[k:]
			}
		}
		fmt.Fprintf(&s, "\n    %s\n%s  %s", string(rs), strings.Repeat(" ", 3+j)+"^", strings.TrimPrefix(er.Error(), "json: "))
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
	fmt.Fprintf(&s, "    %s\n    ^  %s", ss.String(), msg)
	return s.String()
}

func toValidUTF8(s string) string {
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return s
}
