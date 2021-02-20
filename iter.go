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

// SliceIter is a Iter that iterate a interface{} slice from first to last value
type SliceIter struct{ Slice []interface{} }

// Next value in slice or no value if at end
func (i *SliceIter) Next() (interface{}, bool) {
	if len(i.Slice) == 0 {
		return nil, false
	}
	e := i.Slice[0]
	i.Slice = i.Slice[1:]
	return e, true
}

// EmptyIter is a Iter that return no value, similar to "empty" keyword.
type EmptyIter struct{}

// Next returns no value
func (EmptyIter) Next() (interface{}, bool) { return nil, false }

// IterFn is a Iter that calls a provided next function
type IterFn func() (interface{}, bool)

// Next value in slice or no value if at end
func (i IterFn) Next() (interface{}, bool) { return i() }
