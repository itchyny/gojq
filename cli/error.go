package cli

import (
	"encoding/json"
	"fmt"
	"io"
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
	fname, contents string
	err             error
}

func (err *queryParseError) Error() string {
	var offset int
	if e, ok := err.err.(interface{ Token() (string, int) }); ok {
		var token string
		token, offset = e.Token()
		offset -= len(token) - 1
	}
	linestr, line, column := getLineByOffset(err.contents, offset)
	if err.fname != "<arg>" || containsNewline(err.contents) {
		return fmt.Sprintf("invalid query: %s:%d\n%s  %s",
			err.fname, line, formatLineInfo(linestr, line, column), err.err)
	}
	return fmt.Sprintf("invalid query: %s\n    %s\n%s    ^  %s",
		err.contents, linestr, strings.Repeat(" ", column), err.err)
}

func (err *queryParseError) ExitCode() int {
	return exitCodeCompileErr
}

type jsonParseError struct {
	fname, contents string
	line            int
	err             error
}

func (err *jsonParseError) Error() string {
	var offset int
	if err.err == io.ErrUnexpectedEOF {
		offset = len(err.contents) + 1
	} else if e, ok := err.err.(*json.SyntaxError); ok {
		offset = int(e.Offset)
	}
	linestr, line, column := getLineByOffset(err.contents, offset)
	if line += err.line; line > 1 {
		return fmt.Sprintf("invalid json: %s:%d\n%s  %s",
			err.fname, line, formatLineInfo(linestr, line, column), err.err)
	}
	return fmt.Sprintf("invalid json: %s\n    %s\n%s    ^  %s",
		err.fname, linestr, strings.Repeat(" ", column), err.err)
}

type yamlParseError struct {
	fname, contents string
	err             error
}

func (err *yamlParseError) Error() string {
	var line int
	msg := strings.TrimPrefix(
		strings.TrimPrefix(err.err.Error(), "yaml: "),
		"unmarshal errors:\n  ")
	if fmt.Sscanf(msg, "line %d: ", &line); line == 0 {
		return "invalid yaml: " + err.fname
	}
	msg = msg[strings.Index(msg, ": ")+2:]
	if i := strings.IndexByte(msg, '\n'); i >= 0 {
		msg = msg[:i]
	}
	linestr := getLineByLine(err.contents, line)
	return fmt.Sprintf("invalid yaml: %s:%d\n%s  %s",
		err.fname, line, formatLineInfo(linestr, line, 0), msg)
}

func getLineByOffset(str string, offset int) (linestr string, line, column int) {
	ss := &stringScanner{str, 0}
	for {
		str, start, ok := ss.next()
		if !ok {
			offset -= start
			break
		}
		line++
		linestr = str
		if ss.offset >= offset {
			offset -= start
			break
		}
	}
	if offset > len(linestr) {
		offset = len(linestr)
	} else if offset > 0 {
		offset--
	} else {
		offset = 0
	}
	if offset > 48 {
		skip := len(trimLastInvalidRune(linestr[:offset-48]))
		linestr = linestr[skip:]
		offset -= skip
	}
	if len(linestr) > 64 {
		linestr = linestr[:64]
	}
	linestr = trimLastInvalidRune(linestr)
	if offset >= len(linestr) {
		offset = len(linestr)
	} else {
		offset = len(trimLastInvalidRune(linestr[:offset]))
	}
	column = runewidth.StringWidth(linestr[:offset])
	return
}

func getLineByLine(str string, line int) (linestr string) {
	ss := &stringScanner{str, 0}
	for {
		str, _, ok := ss.next()
		if !ok {
			break
		}
		if line--; line == 0 {
			linestr = str
			break
		}
	}
	if len(linestr) > 64 {
		linestr = trimLastInvalidRune(linestr[:64])
	}
	return
}

func trimLastInvalidRune(s string) string {
	for i := len(s) - 1; i >= 0 && i > len(s)-utf8.UTFMax; i-- {
		if b := s[i]; b < utf8.RuneSelf {
			return s[:i+1]
		} else if utf8.RuneStart(b) {
			if r, _ := utf8.DecodeRuneInString(s[i:]); r == utf8.RuneError {
				return s[:i]
			}
			break
		}
	}
	return s
}

func formatLineInfo(linestr string, line, column int) string {
	l := strconv.Itoa(line)
	return "    " + l + " | " + linestr + "\n" +
		strings.Repeat(" ", len(l)+column) + "       ^"
}

type stringScanner struct {
	str    string
	offset int
}

func (ss *stringScanner) next() (line string, start int, ok bool) {
	if ss.offset == len(ss.str) {
		return
	}
	start, ok = ss.offset, true
	line = ss.str[start:]
	i := indexNewline(line)
	if i < 0 {
		ss.offset = len(ss.str)
		return
	}
	line = line[:i]
	if strings.HasPrefix(ss.str[start+i:], "\r\n") {
		i++
	}
	ss.offset += i + 1
	return
}

// Faster than strings.ContainsAny(str, "\r\n").
func containsNewline(str string) bool {
	return strings.IndexByte(str, '\n') >= 0 ||
		strings.IndexByte(str, '\r') >= 0
}

// Faster than strings.IndexAny(str, "\r\n").
func indexNewline(str string) (i int) {
	if i = strings.IndexByte(str, '\n'); i >= 0 {
		str = str[:i]
	}
	if j := strings.IndexByte(str, '\r'); j >= 0 {
		i = j
	}
	return
}
