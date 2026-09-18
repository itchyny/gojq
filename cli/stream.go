package cli

import (
	"encoding/json"
	"errors"
	"io"
)

type jsonStream struct {
	dec    *json.Decoder
	path   []any
	states []int
}

func newJSONStream(dec *json.Decoder) *jsonStream {
	return &jsonStream{dec: dec, states: []int{jsonStateTopValue}, path: []any{}}
}

const (
	jsonStateTopValue = iota
	jsonStateArrayStart
	jsonStateArrayValue
	jsonStateArrayEnd
	jsonStateArrayEmptyEnd
	jsonStateObjectStart
	jsonStateObjectKey
	jsonStateObjectValue
	jsonStateObjectEnd
	jsonStateObjectEmptyEnd
)

func (s *jsonStream) next() (any, error) {
	switch s.states[len(s.states)-1] {
	case jsonStateArrayEnd, jsonStateObjectEnd:
		s.path = s.path[:len(s.path)-1]
		fallthrough
	case jsonStateArrayEmptyEnd, jsonStateObjectEmptyEnd:
		s.states = s.states[:len(s.states)-1]
	}
	if s.dec.More() {
		switch s.states[len(s.states)-1] {
		case jsonStateArrayValue:
			s.path[len(s.path)-1] = s.path[len(s.path)-1].(int) + 1
		case jsonStateObjectValue:
			s.path = s.path[:len(s.path)-1]
		}
	}
	for {
		token, err := s.dec.Token()
		if err != nil {
			if s.states[len(s.states)-1] != jsonStateTopValue && isUnexpectedEOF(err, s.dec) {
				err = io.ErrUnexpectedEOF
			}
			return nil, err
		}
		if d, ok := token.(json.Delim); ok {
			switch d {
			case '[', '{':
				switch s.states[len(s.states)-1] {
				case jsonStateArrayStart:
					s.states[len(s.states)-1] = jsonStateArrayValue
				case jsonStateObjectKey:
					s.states[len(s.states)-1] = jsonStateObjectValue
				}
				if d == '[' {
					s.states = append(s.states, jsonStateArrayStart)
					s.path = append(s.path, 0)
				} else {
					s.states = append(s.states, jsonStateObjectStart)
				}
			case ']':
				if s.states[len(s.states)-1] == jsonStateArrayStart {
					s.states[len(s.states)-1] = jsonStateArrayEmptyEnd
					s.path = s.path[:len(s.path)-1]
					return []any{s.copyPath(), []any{}}, nil
				}
				s.states[len(s.states)-1] = jsonStateArrayEnd
				return []any{s.copyPath()}, nil
			case '}':
				if s.states[len(s.states)-1] == jsonStateObjectStart {
					s.states[len(s.states)-1] = jsonStateObjectEmptyEnd
					return []any{s.copyPath(), map[string]any{}}, nil
				}
				s.states[len(s.states)-1] = jsonStateObjectEnd
				return []any{s.copyPath()}, nil
			}
		} else {
			switch s.states[len(s.states)-1] {
			case jsonStateArrayStart:
				s.states[len(s.states)-1] = jsonStateArrayValue
				fallthrough
			case jsonStateArrayValue:
				return []any{s.copyPath(), token}, nil
			case jsonStateObjectStart, jsonStateObjectValue:
				s.states[len(s.states)-1] = jsonStateObjectKey
				s.path = append(s.path, token)
			case jsonStateObjectKey:
				s.states[len(s.states)-1] = jsonStateObjectValue
				return []any{s.copyPath(), token}, nil
			default:
				s.states[len(s.states)-1] = jsonStateTopValue
				return []any{s.copyPath(), token}, nil
			}
		}
	}
}

func (s *jsonStream) copyPath() []any {
	path := make([]any, len(s.path))
	copy(path, s.path)
	return path
}

// isUnexpectedEOF reports whether err from (*json.Decoder).Token marks the input
// being truncated in the middle of a value. Before Go 1.27 the decoder returned
// io.EOF at this point; since Go 1.27 (encoding/json v2) it instead returns a
// *json.SyntaxError ("unexpected end of JSON input") whose offset sits at the end
// of the consumed input. Both are normalized to io.ErrUnexpectedEOF so callers
// render the same error position and message across Go versions.
func isUnexpectedEOF(err error, dec *json.Decoder) bool {
	if err == io.EOF {
		return true
	}
	var se *json.SyntaxError
	return errors.As(err, &se) && se.Offset >= dec.InputOffset()
}
