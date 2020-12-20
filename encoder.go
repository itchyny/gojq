package gojq

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

func jsonMarshal(v interface{}) string {
	var s strings.Builder
	(&encoder{w: &s}).encode(v)
	return s.String()
}

type encoder struct {
	w   *strings.Builder
	buf [64]byte
}

func (e *encoder) encode(v interface{}) {
	switch v := v.(type) {
	case nil:
		e.w.WriteString("null")
	case bool:
		if v {
			e.w.WriteString("true")
		} else {
			e.w.WriteString("false")
		}
	case int:
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
	case float64:
		e.encodeFloat64(v)
	case *big.Int:
		e.w.Write(v.Append(e.buf[:0], 10))
	case string:
		e.encodeString(v)
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
		e.w.WriteString("null")
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
	for i, v := range vs {
		if i > 0 {
			e.w.WriteByte(',')
		}
		e.encode(v)
	}
	e.w.WriteByte(']')
}

func (e *encoder) encodeMap(vs map[string]interface{}) {
	e.w.WriteByte('{')
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
		e.encodeString(kv.key)
		e.w.WriteByte(':')
		e.encode(kv.val)
	}
	e.w.WriteByte('}')
}
