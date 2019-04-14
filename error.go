package gojq

import (
	"encoding/json"
	"fmt"
	"reflect"
)

type unexpectedQueryError struct {
	q *Query
}

func (err *unexpectedQueryError) Error() string {
	return fmt.Sprintf("unexpected query: %v", err.q)
}

type expectedObjectError struct {
	v interface{}
}

func (err *expectedObjectError) Error() string {
	return fmt.Sprintf("expected an object but got: %s", typeErrorPreview(err.v))
}

type expectedArrayError struct {
	v interface{}
}

func (err *expectedArrayError) Error() string {
	return fmt.Sprintf("expected an array but got: %s", typeErrorPreview(err.v))
}

type iteratorError struct {
	v interface{}
}

func (err *iteratorError) Error() string {
	return fmt.Sprintf("cannot iterate over: %s", typeErrorPreview(err.v))
}

func typeErrorPreview(v interface{}) string {
	return typeof(v) + preview(v)
}

func typeof(v interface{}) (s string) {
	k := reflect.TypeOf(v).Kind()
	switch k {
	case reflect.Array, reflect.Slice:
		return "array"
	case reflect.Map:
		return "object"
	case reflect.Bool:
		return "boolean"
	case reflect.Int, reflect.Uint, reflect.Float64:
		return "number"
	default:
		return k.String()
	}
}

func preview(v interface{}) string {
	bs, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	s, l := string(bs), 25
	if len(s) > l {
		s = s[:l-3] + " ..."
	}
	return " (" + s + ")"
}
