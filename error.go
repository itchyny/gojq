package gojq

import (
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
	return fmt.Sprintf("expected an object but got: %s", typeof(err.v))
}

type expectedArrayError struct {
	v interface{}
}

func (err *expectedArrayError) Error() string {
	return fmt.Sprintf("expected an array but got: %s", typeof(err.v))
}

type iteratorError struct {
	v interface{}
}

func (err *iteratorError) Error() string {
	return fmt.Sprintf("cannot iterate over: %s", typeof(err.v))
}

func typeof(v interface{}) string {
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
