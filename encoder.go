package gojq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/modopayments/go-modo/v8"
	"github.com/modopayments/go-modo/v8/uuid"
)

// Marshal returns the jq-flavored JSON encoding of v.
//
// This method accepts only limited types (nil, bool, int, float64, *big.Int,
// string, []any, and map[string]any) because these are the possible types a
// gojq iterator can emit. This method marshals NaN to null, truncates
// infinities to (+|-) math.MaxFloat64, uses \b and \f in strings, and does not
// escape '<', '>', '&', '\u2028', and '\u2029'. These behaviors are based on
// the marshaler of jq command, and different from json.Marshal in the Go
// standard library. Note that the result is not safe to embed in HTML.
func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	(&encoder{w: &b}).encode(v)
	return b.Bytes(), nil
}

func jsonMarshal(v any) string {
	var sb strings.Builder
	(&encoder{w: &sb}).encode(v)
	return sb.String()
}

func jsonEncodeString(sb *strings.Builder, v string) {
	(&encoder{w: sb}).encodeString(v)
}

type encoder struct {
	w interface {
		io.Writer
		io.ByteWriter
		io.StringWriter
	}
	buf [64]byte
}

func (e *encoder) encode(v any) {
	switch v := v.(type) {
	case nil:
		e.w.WriteString("null")
	case bool:
		if v {
			e.w.WriteString("true")
		} else {
			e.w.WriteString("false")
		}
	case uint:
		e.w.Write(strconv.AppendUint(e.buf[:0], uint64(v), 10))
	case uint8:
		e.w.Write(strconv.AppendUint(e.buf[:0], uint64(v), 10))
	case uint16:
		e.w.Write(strconv.AppendUint(e.buf[:0], uint64(v), 10))
	case uint32:
		e.w.Write(strconv.AppendUint(e.buf[:0], uint64(v), 10))
	case uint64:
		e.w.Write(strconv.AppendUint(e.buf[:0], v, 10))
	case int:
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
	case int8:
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
	case int16:
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
	case int32:
		e.w.Write(strconv.AppendInt(e.buf[:0], int64(v), 10))
	case int64:
		e.w.Write(strconv.AppendInt(e.buf[:0], v, 10))
	case float32:
		e.encodeFloat64(float64(v))
	case float64:
		e.encodeFloat64(v)
	case *big.Int:
		e.w.Write(v.Append(e.buf[:0], 10))
	case string:
		e.encodeString(v)
	case []any:
		e.encodeArray(v)
	case map[string]any:
		e.encodeObject(v)
	case uuid.UUID:
		e.encodeString(v.String())
	case uuid.NullUUID:
		if v.Valid {
			e.encodeString(v.UUID.String())
		} else {
			e.w.WriteString("null")
		}
	case time.Time:
		e.w.Write(strconv.AppendInt(e.buf[:0], v.Unix(), 10))
	case modo.Timestamp:
		e.w.Write(strconv.AppendInt(e.buf[:0], v.Unix(), 10))
	default:
		value := reflect.ValueOf(v)
		switch value.Kind() {
		case reflect.Ptr:
			if value.IsNil() {
				e.encode(nil)
				break
			}
			e.encode(value.Elem().Interface())
		case reflect.Struct:
			e.encodeStruct(value)
		case reflect.Slice: // this an interface{} that happens to mask a []any
			e.encodeValueSlice(value)
		default:
			panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
		}
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
	e.w.Write(buf)
}

// ref: encodeState#string in encoding/json
func (e *encoder) encodeString(s string) {
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
}

func (e *encoder) encodeArray(vs []any) {
	e.w.WriteByte('[')
	for i, v := range vs {
		if i > 0 {
			e.w.WriteByte(',')
		}
		e.encode(v)
	}
	e.w.WriteByte(']')
}

func (e *encoder) encodeObject(vs map[string]any) {
	e.w.WriteByte('{')
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
			e.w.WriteByte(',')
		}
		e.encodeString(kv.key)
		e.w.WriteByte(':')
		e.encode(kv.val)
	}
	e.w.WriteByte('}')
}

func (e *encoder) encodeStruct(vs reflect.Value) {
	enc := json.NewEncoder(e.w)
	enc.Encode(vs.Interface())
}

func (e *encoder) encodeValueSlice(vs reflect.Value) {
	e.w.WriteByte('[')
	for i := 0; i < vs.Len(); i++ {
		if i > 0 {
			e.w.WriteByte(',')
		}
		v := vs.Index(i).Interface()
		e.encode(v)
	}
	e.w.WriteByte(']')
}
