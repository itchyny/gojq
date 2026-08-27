package gojq

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/big"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Marshal returns the jq-flavored JSON encoding of v.
//
// This method accepts only limited types (nil, bool, int, float64, *big.Int,
// json.Number, string, []any, and map[string]any) because these are the
// possible types a gojq iterator can emit. This method marshals NaN to null,
// truncates infinities to (+|-) math.MaxFloat64, uses \b and \f in strings,
// and does not escape '<', '>', '&', '\u2028', and '\u2029'. These behaviors
// are based on the marshaler of jq command, and different from json.Marshal in
// the Go standard library. Note that the result is not safe to embed in HTML.
func Marshal(v any) (bs []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*allocLimitError); ok {
				bs, err = nil, &allocLimitError{}
				return
			}
			panic(r)
		}
	}()
	var b bytes.Buffer
	(&encoder{w: &b}).encode(v)
	return b.Bytes(), nil
}

func jsonMarshal(v any) string {
	var sb strings.Builder
	(&encoder{w: &sb}).encode(v)
	return sb.String()
}

// jsonMarshalTruncated is jsonMarshal but returns the bounded prefix built so far
// instead of letting the encode guard's *allocLimitError panic escape. Error() and
// String() methods cannot return an error, so a big value would otherwise crash
// formatting. jsonMarshal itself must keep panicking, since marshalBounded relies
// on that to report the limit as an error.
func jsonMarshalTruncated(v any) (s string) {
	var sb strings.Builder
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*allocLimitError); !ok {
				panic(r)
			}
			s = sb.String()
		}
	}()
	(&encoder{w: &sb}).encode(v)
	return sb.String()
}

// marshalBounded is jsonMarshal, but returns an allocLimitError instead of
// building an encoding larger than MaxAlloc (see encode).
func marshalBounded(v any) (s string, err error) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*allocLimitError); ok {
				err = &allocLimitError{}
				return
			}
			panic(r)
		}
	}()
	return jsonMarshal(v), nil
}

func jsonEncodeString(sb *strings.Builder, v string) {
	(&encoder{w: sb}).encodeString(v)
}

type encoder struct {
	w interface {
		io.Writer
		io.ByteWriter
		io.StringWriter
		Len() int
	}
	buf   [64]byte
	depth int
}

func (e *encoder) encode(v any) {
	// Shared references (e.g. `[., .]` repeated) make a value whose own
	// footprint is tiny expand to an exponentially larger encoding. Bound the
	// output so tojson / tostring / @json cannot build gigabytes before the
	// value meter, which only sees the finished string, could charge it.
	if MaxAlloc > 0 && int64(e.w.Len()) > MaxAlloc {
		panic(&allocLimitError{})
	}
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
		// Converting a big integer to base 10 allocates large superlinear scratch
		// inside math/big (a 4 MB integer peaks past 100 MB), none of it visible to
		// the between-values guard above. Reject one whose decimal form could pass
		// the limit; the widest realistic integer is tiny, so this only refuses
		// absurd multi-hundred-thousand-digit values.
		if MaxAlloc > 0 && int64(len(v.Bits()))*384 > MaxAlloc {
			panic(&allocLimitError{})
		}
		e.w.Write(v.Append(e.buf[:0], 10))
	case json.Number:
		e.w.WriteString(v.String())
	case string:
		e.encodeString(v)
	case []any:
		e.encodeArray(v)
	case map[string]any:
		e.encodeObject(v)
	default:
		panic(fmt.Sprintf("invalid type: %[1]T (%[1]v)", v))
	}
}

// ref: floatEncoder in encoding/json
func (e *encoder) encodeFloat64(f float64) {
	if math.IsNaN(f) {
		e.w.WriteString("null")
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
			if MaxAlloc > 0 && int64(e.w.Len()) > MaxAlloc {
				panic(&allocLimitError{})
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
			if MaxAlloc > 0 && int64(e.w.Len()) > MaxAlloc {
				panic(&allocLimitError{})
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
	// Bound the recursion depth. encode is Go-recursive and is not covered by
	// the interpreter's overStackLimit, and the buffer guard does not fire on a
	// deep-narrow value ( each level adds one byte on the way down ), so a deeply
	// nested value overflowed the goroutine stack ( a fatal, unrecoverable crash ).
	// A value this deep is already larger than the limit ( >= 16 bytes per level ).
	if e.depth++; MaxAlloc > 0 && (int64(e.depth)*16 > MaxAlloc || e.depth > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	defer func() { e.depth-- }()
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
	if e.depth++; MaxAlloc > 0 && (int64(e.depth)*16 > MaxAlloc || e.depth > maxRecursionDepth) {
		panic(&allocLimitError{})
	}
	defer func() { e.depth-- }()
	e.w.WriteByte('{')
	type keyVal struct {
		key string
		val any
	}
	if MaxAlloc > 0 && int64(len(vs))*32 > MaxAlloc {
		panic(&allocLimitError{})
	}
	kvs := make([]keyVal, len(vs))
	var i int
	for k, v := range vs {
		kvs[i] = keyVal{k, v}
		i++
	}
	slices.SortFunc(kvs, func(x, y keyVal) int {
		return cmp.Compare(x.key, y.key)
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
