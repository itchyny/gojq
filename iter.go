package gojq

// Iter is an interface for an iterator.
type Iter interface {
	Next() (interface{}, bool)
}

func unitIterator(v interface{}) Iter {
	return &unitIter{v: v}
}

type unitIter struct {
	v    interface{}
	done bool
}

func (iter *unitIter) Next() (interface{}, bool) {
	if iter.done {
		return nil, false
	}
	iter.done = true
	return iter.v, true
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
