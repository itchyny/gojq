package cli

import (
	"encoding/json"
	"io"
)

type jsonStream struct {
	dec    *json.Decoder
	path   []interface{}
	states []int
}

func newJSONStream(dec *json.Decoder) *jsonStream {
	return &jsonStream{dec: dec, states: []int{jsonStateTopValue}, path: []interface{}{}}
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

func (s *jsonStream) next() (interface{}, error) {
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
			if err == io.EOF && s.states[len(s.states)-1] != jsonStateTopValue {
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
					return []interface{}{s.copyPath(), []interface{}{}}, nil
				}
				s.states[len(s.states)-1] = jsonStateArrayEnd
				return []interface{}{s.copyPath()}, nil
			case '}':
				if s.states[len(s.states)-1] == jsonStateObjectStart {
					s.states[len(s.states)-1] = jsonStateObjectEmptyEnd
					return []interface{}{s.copyPath(), map[string]interface{}{}}, nil
				}
				s.states[len(s.states)-1] = jsonStateObjectEnd
				return []interface{}{s.copyPath()}, nil
			default:
				panic(d)
			}
		} else {
			switch s.states[len(s.states)-1] {
			case jsonStateArrayStart:
				s.states[len(s.states)-1] = jsonStateArrayValue
				fallthrough
			case jsonStateArrayValue:
				return []interface{}{s.copyPath(), token}, nil
			case jsonStateObjectStart, jsonStateObjectValue:
				s.states[len(s.states)-1] = jsonStateObjectKey
				s.path = append(s.path, token)
			case jsonStateObjectKey:
				s.states[len(s.states)-1] = jsonStateObjectValue
				return []interface{}{s.copyPath(), token}, nil
			default:
				s.states[len(s.states)-1] = jsonStateTopValue
				return []interface{}{s.copyPath(), token}, nil
			}
		}
	}
}

func (s *jsonStream) copyPath() []interface{} {
	path := make([]interface{}, len(s.path))
	copy(path, s.path)
	return path
}
