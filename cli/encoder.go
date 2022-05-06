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
)

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
		e.write([]byte("null"), nullColor)
	case bool:
		if v {
			e.write([]byte("true"), trueColor)
		} else {
			e.write([]byte("false"), falseColor)
		}
	case int:
		e.write(strconv.AppendInt(e.buf[:0], int64(v), 10), numberColor)
	case float64:
		e.encodeFloat64(v)
	case *big.Int:
		e.write(v.Append(e.buf[:0], 10), numberColor)
	case string:
		e.encodeString(v, stringColor)
	case []interface{}:
		e.encodeArray(v)
	case map[string]interface{}:
		e.encodeMap(v)
	default:
		panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
	}
	if e.w.Len() > 8*1024 {
		e.out.Write(e.w.Bytes())
		e.w.Reset()
	}
}

// ref: floatEncoder in encoding/json
func (e *encoder) encodeFloat64(f float64) {
	if math.IsNaN(f) {
		e.write([]byte("null"), nullColor)
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
	e.write(buf, numberColor)
}

// ref: encodeState#string in encoding/json
func (e *encoder) encodeString(s string, color []byte) {
	if color != nil {
		setColor(e.w, color)
	}
	e.w.WriteByte('"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if ' ' <= b && b <= '~' && b != '"' && b != '\\' {
				i++
				continue
			}
			if start < i {
				e.w.WriteString(s[start:i])
			}
			switch b {
			case '"':
				e.w.WriteString(`\"`)
			case '\\':
				e.w.WriteString(`\\`)
			case '\b':
				e.w.WriteString(`\b`)
			case '\f':
				e.w.WriteString(`\f`)
			case '\n':
				e.w.WriteString(`\n`)
			case '\r':
				e.w.WriteString(`\r`)
			case '\t':
				e.w.WriteString(`\t`)
			default:
				const hex = "0123456789abcdef"
				e.w.WriteString(`\u00`)
				e.w.WriteByte(hex[b>>4])
				e.w.WriteByte(hex[b&0xF])
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
	if color != nil {
		setColor(e.w, resetColor)
	}
}

func (e *encoder) encodeArray(vs []interface{}) {
	e.writeByte('[', arrayColor)
	e.depth += e.indent
	for i, v := range vs {
		if i > 0 {
			e.writeByte(',', arrayColor)
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
	e.writeByte(']', arrayColor)
}

func (e *encoder) encodeMap(vs map[string]interface{}) {
	e.writeByte('{', objectColor)
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
			e.writeByte(',', objectColor)
		}
		if e.indent != 0 {
			e.writeIndent()
		}
		e.encodeString(kv.key, objectKeyColor)
		e.writeByte(':', objectColor)
		if e.indent != 0 {
			e.w.WriteByte(' ')
		}
		e.encode(kv.val)
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.writeByte('}', objectColor)
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

func (e *encoder) writeByte(b byte, color []byte) {
	if color == nil {
		e.w.WriteByte(b)
	} else {
		setColor(e.w, color)
		e.w.WriteByte(b)
		setColor(e.w, resetColor)
	}
}

func (e *encoder) write(bs []byte, color []byte) {
	if color == nil {
		e.w.Write(bs)
	} else {
		setColor(e.w, color)
		e.w.Write(bs)
		setColor(e.w, resetColor)
	}
}
