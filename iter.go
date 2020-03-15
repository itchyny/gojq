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

func (c *unitIter) Next() (interface{}, bool) {
	if !c.done {
		c.done = true
		return c.v, true
	}
	return nil, false
}
