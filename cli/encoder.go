package cli

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"math/big"
	"sort"
	"strconv"
	"unicode/utf8"
	_ "unsafe" // go:linkname

	"github.com/fatih/color"
)

var (
	nullColor   = color.New(color.FgHiBlack)
	boolColor   = color.New(color.FgYellow)
	numberColor = color.New(color.FgCyan)
	stringColor = color.New(color.FgGreen)
	keyColor    = color.New(color.FgBlue, color.Bold)
)

//go:linkname setColor github.com/fatih/color.(*Color).setWriter
func setColor(*color.Color, io.Writer) *color.Color

//go:linkname unsetColor github.com/fatih/color.(*Color).unsetWriter
func unsetColor(*color.Color, io.Writer)

type encoder struct {
	out    io.Writer
	w      *bytes.Buffer
	tab    bool
	indent int
	depth  int
	buf    [64]byte
}

func newEncoder(tab bool, indent int) *encoder {
	// reuse the buffer in multiple calls of marshal
	return &encoder{w: new(bytes.Buffer), tab: tab, indent: indent}
}

func (e *encoder) marshal(v interface{}, w io.Writer) error {
	e.out = w
	e.encode(v)
	_, err := w.Write(e.w.Bytes())
	e.w.Reset()
	return err
}

func (e *encoder) encode(v interface{}) {
	switch v := v.(type) {
	case nil:
		setColor(nullColor, e.w)
		e.w.WriteString("null")
		unsetColor(nullColor, e.w)
	case bool:
		setColor(boolColor, e.w)
		if v {
			e.w.WriteString("true")
		} else {
			e.w.WriteString("false")
		}
		unsetColor(boolColor, e.w)
	case int:
		setColor(numberColor, e.w)
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
		unsetColor(numberColor, e.w)
	case float64:
		e.encodeFloat64(v)
	case *big.Int:
		setColor(numberColor, e.w)
		e.w.Write(v.Append(e.buf[:0], 10))
		unsetColor(numberColor, e.w)
	case string:
		setColor(stringColor, e.w)
		e.encodeString(v)
		unsetColor(stringColor, e.w)
	case []interface{}:
		e.encodeArray(v)
	case map[string]interface{}:
		e.encodeMap(v)
	default:
		panic(fmt.Sprintf("invalid value: %v", v))
	}
	if e.w.Len() > 8*1024 {
		e.out.Write(e.w.Bytes())
		e.w.Reset()
	}
}

// ref: floatEncoder in encoding/json
func (e *encoder) encodeFloat64(f float64) {
	if math.IsNaN(f) {
		setColor(nullColor, e.w)
		e.w.WriteString("null")
		unsetColor(nullColor, e.w)
		return
	}
	setColor(numberColor, e.w)
	if f >= math.MaxFloat64 {
		f = math.MaxFloat64
	} else if f <= -math.MaxFloat64 {
		f = -math.MaxFloat64
	}
	fmt := byte('f')
	if x := math.Abs(f); x != 0 && x < 1e-6 || x >= 1e21 {
		fmt = 'e'
	}
	buf := strconv.AppendFloat(e.buf[:0], f, fmt, -1, 64)
	if fmt == 'e' {
		// clean up e-09 to e-9
		if n := len(buf); n >= 4 && buf[n-4] == 'e' && buf[n-3] == '-' && buf[n-2] == '0' {
			buf[n-2] = buf[n-1]
			buf = buf[:n-1]
		}
	}
	e.w.Write(buf)
	unsetColor(numberColor, e.w)
}

// ref: encodeState#string in encoding/json
func (e *encoder) encodeString(s string) {
	e.w.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if ']' <= b && b <= '~' || '#' <= b && b <= '[' || b == ' ' || b == '!' {
				i++
				continue
			}
			if start < i {
				e.w.WriteString(s[start:i])
			}
			e.w.WriteByte('\\')
			switch b {
			case '\\', '"':
				e.w.WriteByte(b)
			case '\n':
				e.w.WriteByte('n')
			case '\r':
				e.w.WriteByte('r')
			case '\t':
				e.w.WriteByte('t')
			default:
				const hex = "0123456789abcdef"
				e.w.Write([]byte{'u', '0', '0', hex[b>>4], hex[b&0xF]})
			}
			i++
			start = i
			continue
		}
		c, size := utf8.DecodeRuneInString(s[i:])
		if c == utf8.RuneError && size == 1 {
			if start < i {
				e.w.WriteString(s[start:i])
			}
			e.w.WriteString(`\ufffd`)
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		e.w.WriteString(s[start:])
	}
	e.w.WriteByte('"')
}

func (e *encoder) encodeArray(vs []interface{}) {
	e.w.WriteByte('[')
	e.depth += e.indent
	for i, v := range vs {
		if i > 0 {
			e.w.WriteByte(',')
		}
		if e.indent != 0 {
			e.writeIndent()
		}
		e.encode(v)
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.w.WriteByte(']')
}

func (e *encoder) encodeMap(vs map[string]interface{}) {
	e.w.WriteByte('{')
	e.depth += e.indent
	type keyVal struct {
		key string
		val interface{}
	}
	kvs := make([]keyVal, len(vs))
	var i int
	for k, v := range vs {
		kvs[i] = keyVal{k, v}
		i++
	}
	sort.Slice(kvs, func(i, j int) bool {
		return kvs[i].key < kvs[j].key
	})
	for i, kv := range kvs {
		if i > 0 {
			e.w.WriteByte(',')
		}
		if e.indent != 0 {
			e.writeIndent()
		}
		setColor(keyColor, e.w)
		e.encodeString(kv.key)
		unsetColor(keyColor, e.w)
		if e.indent == 0 {
			e.w.WriteByte(':')
		} else {
			e.w.Write([]byte{':', ' '})
		}
		e.encode(kv.val)
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.w.WriteByte('}')
}

func (e *encoder) writeIndent() {
	e.w.WriteByte('\n')
	if n := e.depth; n > 0 {
		if e.tab {
			const tabs = "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t"
			for n > len(tabs) {
				e.w.Write([]byte(tabs))
				n -= len(tabs)
			}
			e.w.Write([]byte(tabs)[:n])
		} else {
			const spaces = "                                                                "
			for n > len(spaces) {
				e.w.Write([]byte(spaces))
				n -= len(spaces)
			}
			e.w.Write([]byte(spaces)[:n])
		}
	}
}
