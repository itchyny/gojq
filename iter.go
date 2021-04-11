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
