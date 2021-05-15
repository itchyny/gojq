package gojq

// Iter is an interface for an iterator.
type Iter interface {
	Next() (interface{}, bool)
}

// NewIter creates a new Iter from values.
func NewIter(values ...interface{}) Iter {
	iter := sliceIter(values)
	return &iter
}

type sliceIter []interface{}

func (iter *sliceIter) Next() (interface{}, bool) {
	if len(*iter) == 0 {
		return nil, false
	}
	value := (*iter)[0]
	*iter = (*iter)[1:]
	return value, true
}
