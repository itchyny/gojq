package gojq

import "fmt"

type unexpectedQueryError struct {
	q *Query
}

func (err *unexpectedQueryError) Error() string {
	return fmt.Sprintf("unexpected query: %v", err.q)
}
