package gojq

import "fmt"

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
	return fmt.Sprintf("expected an object but got: %T", err.v)
}

type expectedArrayError struct {
	v interface{}
}

func (err *expectedArrayError) Error() string {
	return fmt.Sprintf("expected an array but got: %T", err.v)
}
