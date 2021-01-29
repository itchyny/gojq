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
	if er, ok := err.err.(interface{ Token() (string, int) }); ok {
		_, offset := er.Token()
		linestr, line, col := getLineByOffset(err.contents, offset)
		var fname, prefix string
		if !strings.ContainsAny(err.contents, "\n\r") && strings.HasPrefix(err.fname, "<arg>") {
			fname = err.contents
		} else {
			fname = err.fname + ":" + strconv.Itoa(line)
			prefix = strconv.Itoa(line) + " | "
		}
		return "invalid " + err.typ + ": " + fname + "\n" +
			"    " + prefix + linestr + "\n" +
			"    " + strings.Repeat(" ", len(prefix)+col) + "^  " + err.err.Error()
	}
	return "invalid " + err.typ + ": " + err.fname + ": " + err.err.Error()
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
	var prefix, linestr string
	var line, col int
	fname, errmsg := err.fname, err.err.Error()
	if errmsg == "unexpected EOF" {
		linestr = strings.TrimRight(err.contents, "\n\r")
		if i := strings.LastIndexAny(linestr, "\n\r"); i >= 0 {
			linestr = linestr[i:]
		}
		col = runewidth.StringWidth(linestr)
	} else if er, ok := err.err.(*json.SyntaxError); ok {
		linestr, line, col = getLineByOffset(
			trimLastInvalidRune(err.contents), int(er.Offset),
		)
		if i := strings.IndexAny(err.contents, "\n\r"); i >= 0 && i < len(err.contents)-1 {
			line += err.line
			fname += ":" + strconv.Itoa(line)
			prefix = strconv.Itoa(line) + " | "
		}
	}
	return "invalid json: " + fname + "\n" +
		"    " + prefix + linestr + "\n" +
		"    " + strings.Repeat(" ", len(prefix)+col) + "^  " + errmsg
}

type yamlParseError struct {
	fname, contents string
	err             error
}

func (err *yamlParseError) Error() string {
	var line int
	msg := err.err.Error()
	fmt.Sscanf(msg, "yaml: line %d:", &line)
	if line > 0 {
		msg = msg[7+strings.IndexRune(msg[5:], ':'):] // trim "yaml: line N:"
	} else {
		if !strings.HasPrefix(msg, "yaml: unmarshal errors:\n") {
			return "invalid yaml: " + err.fname
		}
		msg = strings.Split(msg, "\n")[1]
		fmt.Sscanf(msg, " line %d: ", &line)
		if line > 0 {
			msg = msg[2+strings.IndexRune(msg, ':'):] // trim "line N:"
		} else {
			return "invalid yaml: " + err.fname
		}
	}
	var ss strings.Builder
	var i, j int
	var cr bool
	for _, r := range trimLastInvalidRune(err.contents) {
		i += len(string(r))
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
	linestr := strconv.Itoa(line)
	return "invalid yaml: " + err.fname + ":" + linestr + "\n" +
		"    " + linestr + " | " + ss.String() + "\n" +
		"    " + strings.Repeat(" ", len(linestr)) + "   ^  " + msg
}

func getLineByOffset(str string, offset int) (string, int, int) {
	var pos, col int
	var cr bool
	line, total := 1, len(str)
	for offset > 128 && offset <= total {
		diff := offset / 2
		for i := 0; i < utf8.UTFMax; i++ {
			if r, _ := utf8.DecodeLastRuneInString(str[:diff+i]); r != utf8.RuneError {
				diff += i
				break
			}
		}
		for _, r := range str[:diff] {
			if r == '\n' || r == '\r' {
				if !cr || r != '\n' {
					line++
				}
				cr = r == '\r'
			}
		}
		str = str[diff:]
		offset -= diff
	}
	var ss strings.Builder
	for _, r := range str {
		if k := utf8.RuneLen(r); k > 0 {
			pos += k
		} else {
			pos += len(string(r))
		}
		if pos < offset {
			col += runewidth.RuneWidth(r)
		}
		if r == '\n' || r == '\r' {
			if pos >= offset {
				break
			} else if pos < total {
				col = 0
				if !cr || r != '\n' {
					line++
				}
				cr = r == '\r'
				ss.Reset()
			}
		} else {
			cr = false
			ss.WriteRune(r)
			if ss.Len() > 64 {
				if pos > offset {
					break
				}
				s, i := ss.String(), 48
				ss.Reset()
				for j := 0; j < utf8.UTFMax; j++ {
					if r, _ := utf8.DecodeRuneInString(s[i+j:]); r != utf8.RuneError {
						i += j
						break
					}
				}
				col -= runewidth.StringWidth(s[:i])
				ss.WriteString(s[i:])
			}
		}
	}
	return ss.String(), line, col
}

func trimLastInvalidRune(s string) string {
	for i := 0; i < utf8.UTFMax && i < len(s); i++ {
		if r, _ := utf8.DecodeLastRuneInString(s[:len(s)-i]); r != utf8.RuneError {
			return s[:len(s)-i]
		}
	}
	return s
}
