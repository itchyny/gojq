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

func (e *encoder) flush() error {
	_, err := e.out.Write(e.w.Bytes())
	e.w.Reset()
	return err
}

func (e *encoder) marshal(v any, w io.Writer) error {
	e.out = w
	err := e.encode(v)
	if ferr := e.flush(); ferr != nil && err == nil {
		err = ferr
	}
	return err
}

func (e *encoder) encode(v any) error {
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
	case []any:
		if err := e.encodeArray(v); err != nil {
			return err
		}
	case map[string]any:
		if err := e.encodeObject(v); err != nil {
			return err
		}
	default:
		panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
	}
	if e.w.Len() > 8*1024 {
		return e.flush()
	}
	return nil
}

// ref: floatEncoder in encoding/json
func (e *encoder) encodeFloat64(f float64) {
	if math.IsNaN(f) {
		e.write([]byte("null"), nullColor)
		return
	}
	f = min(max(f, -math.MaxFloat64), math.MaxFloat64)
	format := byte('f')
	if x := math.Abs(f); x != 0 && x < 1e-6 || x >= 1e21 {
		format = 'e'
	}
	buf := strconv.AppendFloat(e.buf[:0], f, format, -1, 64)
	if format == 'e' {
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

func (e *encoder) encodeArray(vs []any) error {
	e.writeByte('[', arrayColor)
	e.depth += e.indent
	for i, v := range vs {
		if i > 0 {
			e.writeByte(',', arrayColor)
		}
		if e.indent != 0 {
			e.writeIndent()
		}
		if err := e.encode(v); err != nil {
			return err
		}
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.writeByte(']', arrayColor)
	return nil
}

func (e *encoder) encodeObject(vs map[string]any) error {
	e.writeByte('{', objectColor)
	e.depth += e.indent
	type keyVal struct {
		key string
		val any
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
		if err := e.encode(kv.val); err != nil {
			return err
		}
	}
	e.depth -= e.indent
	if len(vs) > 0 && e.indent != 0 {
		e.writeIndent()
	}
	e.writeByte('}', objectColor)
	return nil
}

func (e *encoder) writeIndent() {
	e.w.WriteByte('\n')
	if n := e.depth; n > 0 {
		if e.tab {
			e.writeIndentInternal(n, "\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t\t")
		} else {
			e.writeIndentInternal(n, "                                ")
		}
	}
}

func (e *encoder) writeIndentInternal(n int, spaces string) {
	if l := len(spaces); n <= l {
		e.w.WriteString(spaces[:n])
	} else {
		e.w.WriteString(spaces)
		for n -= l; n > 0; n, l = n-l, l*2 {
			if n < l {
				l = n
			}
			e.w.Write(e.w.Bytes()[e.w.Len()-l:])
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

func (e *encoder) write(bs, color []byte) {
	if color == nil {
		e.w.Write(bs)
	} else {
		setColor(e.w, color)
		e.w.Write(bs)
		setColor(e.w, resetColor)
	}
}
