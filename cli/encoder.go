package cli

import (
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
	w      io.Writer
	indent int
	depth  int
	buf    [64]byte
}

func newEncoder(indent int) *encoder {
	return &encoder{indent: indent}
}

func (e *encoder) marshal(v interface{}, w io.Writer) error {
	e.w = w
	e.encode(v)
	return nil
}

func (e *encoder) encode(v interface{}) {
	switch v := v.(type) {
	case nil:
		setColor(nullColor, e.w)
		e.w.Write([]byte("null"))
		unsetColor(nullColor, e.w)
	case bool:
		setColor(boolColor, e.w)
		if v {
			e.w.Write([]byte("true"))
		} else {
			e.w.Write([]byte("false"))
		}
		unsetColor(boolColor, e.w)
	case int:
		setColor(numberColor, e.w)
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
		unsetColor(numberColor, e.w)
	case float64:
		setColor(numberColor, e.w)
		e.encodeFloat64(v)
		unsetColor(numberColor, e.w)
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
}

// ref: floatEncoder in encoding/json
func (e *encoder) encodeFloat64(f float64) {
	if math.IsNaN(f) {
		e.w.Write([]byte("null"))
		return
	}
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
}

// ref: encodeState#string in encoding/json
func (e *encoder) encodeString(s string) {
	e.w.Write([]byte{'"'})
	start, xs := 0, []byte(s)
	for i := 0; i < len(xs); {
		if b := xs[i]; b < utf8.RuneSelf {
			if ']' <= b && b <= '~' || '#' <= b && b <= '[' || b == ' ' || b == '!' {
				i++
				continue
			}
			if start < i {
				e.w.Write(xs[start:i])
			}
			e.w.Write([]byte{'\\'})
			switch b {
			case '\\', '"':
				e.w.Write([]byte{b})
			case '\n':
				e.w.Write([]byte{'n'})
			case '\r':
				e.w.Write([]byte{'r'})
			case '\t':
				e.w.Write([]byte{'t'})
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
				e.w.Write(xs[start:i])
			}
			e.w.Write([]byte(`\ufffd`))
			i += size
			start = i
			continue
		}
		i += size
	}
	if start < len(s) {
		e.w.Write(xs[start:])
	}
	e.w.Write([]byte{'"'})
}

func (e *encoder) encodeArray(vs []interface{}) {
	e.w.Write([]byte{'['})
	e.depth += e.indent
	for i, v := range vs {
		if i > 0 {
			e.w.Write([]byte{','})
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
	e.w.Write([]byte{']'})
}

func (e *encoder) encodeMap(vs map[string]interface{}) {
	e.w.Write([]byte{'{'})
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
			e.w.Write([]byte{','})
		}
		if e.indent != 0 {
			e.writeIndent()
		}
		setColor(keyColor, e.w)
		e.encodeString(kv.key)
		unsetColor(keyColor, e.w)
		if e.indent == 0 {
			e.w.Write([]byte{':'})
		} else {
			e.w.Write([]byte{':', ' '})
		}
		e.encode(kv.val)
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.w.Write([]byte{'}'})
}

func (e *encoder) writeIndent() {
	e.w.Write([]byte{'\n'})
	const xs = "                                                                "
	if n := e.depth; n > 0 {
		for n > len(xs) {
			e.w.Write([]byte(xs))
			n -= len(xs)
		}
		e.w.Write([]byte(xs)[:n])
	}
}
