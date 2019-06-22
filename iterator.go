package gojq

import "sync"

// Iter ...
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

func indexIterator(get func(int) (interface{}, bool)) Iter {
	return &indexIter{get: get}
}

type indexIter struct {
	get   func(int) (interface{}, bool)
	index int
	done  bool
}

func (c *indexIter) Next() (interface{}, bool) {
	if c.done {
		return nil, false
	}
	v, ok := c.get(c.index)
	if !ok {
		c.done = true
		return nil, false
	}
	c.index++
	return v, true
}

func sliceIterator(xs []interface{}) Iter {
	return indexIterator(func(i int) (interface{}, bool) {
		if i < len(xs) {
			return xs[i], true
		}
		return nil, false
	})
}

func objectIterator(c Iter, keys Iter, values Iter) Iter {
	ks := reuseIterator(keys)
	vs := reuseIterator(values)
	return mapIterator(c, func(v interface{}) interface{} {
		m := v.(map[string]interface{})
		return mapIterator(ks(), func(key interface{}) interface{} {
			k, ok := key.(string)
			if !ok {
				return &objectKeyNotStringError{key}
			}
			return mapIterator(vs(), func(value interface{}) interface{} {
				l := make(map[string]interface{})
				for k, v := range m {
					l[k] = v
				}
				l[k] = value
				return l
			})
		})
	})
}

func objectKeyIterator(c Iter, keys Iter, values Iter) Iter {
	ks := reuseIterator(keys)
	vs := reuseIterator(values)
	return mapIterator(c, func(v interface{}) interface{} {
		m := v.(map[string]interface{})
		return mapIterator(ks(), func(key interface{}) interface{} {
			k, ok := key.(string)
			if !ok {
				return &objectKeyNotStringError{key}
			}
			return mapIterator(vs(), func(value interface{}) interface{} {
				l := make(map[string]interface{})
				for k, v := range m {
					l[k] = v
				}
				v, ok := value.(map[string]interface{})
				if !ok {
					return &expectedObjectError{v}
				}
				l[k] = v[k]
				return l
			})
		})
	})
}

func stringIterator(xs []Iter) Iter {
	if len(xs) == 0 {
		return unitIterator("")
	}
	d := reuseIterator(xs[0])
	return mapIterator(stringIterator(xs[1:]), func(v interface{}) interface{} {
		s := v.(string)
		return mapIterator(d(), func(v interface{}) interface{} {
			switch v := v.(type) {
			case string:
				return v + s
			default:
				return funcToJSON(v).(string) + s
			}
		})
	})
}

func binopIteratorAlt(l Iter, r Iter) Iter {
	return &binopIterAlt{l: l, r: r}
}

type binopIterAlt struct {
	l, r        Iter
	ldone, done bool
}

func (c *binopIterAlt) Next() (interface{}, bool) {
	for !c.done {
		if !c.ldone {
			if v, ok := c.l.Next(); ok {
				if _, ok := v.(error); ok {
					c.done = true
					return v, true
				}
				if v == struct{}{} {
					continue
				}
				if valueToBool(v) {
					c.done = true
					return v, true
				}
				continue
			} else {
				c.ldone = true
			}
		}
		if v, ok := c.r.Next(); ok {
			if v == struct{}{} {
				continue
			}
			return v, true
		}
		c.done = true
	}
	return nil, false
}

func binopIteratorOr(l Iter, r Iter) Iter {
	return &binopOrIter{l: l, riter: reuseIterator(r)}
}

type binopOrIter struct {
	l     Iter
	riter func() Iter
	r     Iter
	done  bool
}

func (c *binopOrIter) Next() (interface{}, bool) {
	for !c.done {
		if c.r != nil {
			if r, ok := c.r.Next(); ok {
				if r == struct{}{} {
					continue
				}
				if err, ok := r.(error); ok {
					c.done = true
					return err, true
				}
				return valueToBool(r), true
			}
			c.r = nil
		}
		l, ok := c.l.Next()
		if !ok {
			c.done = true
			break
		}
		if err, ok := l.(error); ok {
			c.done = true
			return err, ok
		}
		if l == struct{}{} {
			continue
		}
		if valueToBool(l) {
			return true, true
		}
		c.r = c.riter()
	}
	return nil, false
}

func binopIteratorAnd(l Iter, r Iter) Iter {
	return &binopAndIter{l: l, riter: reuseIterator(r)}
}

type binopAndIter struct {
	l     Iter
	riter func() Iter
	r     Iter
	done  bool
}

func (c *binopAndIter) Next() (interface{}, bool) {
	for !c.done {
		if c.r != nil {
			if r, ok := c.r.Next(); ok {
				if r == struct{}{} {
					continue
				}
				if err, ok := r.(error); ok {
					c.done = true
					return err, true
				}
				return valueToBool(r), true
			}
			c.r = nil
		}
		l, ok := c.l.Next()
		if !ok {
			c.done = true
			break
		}
		if err, ok := l.(error); ok {
			c.done = true
			return err, true
		}
		if l == struct{}{} {
			continue
		}
		if !valueToBool(l) {
			return false, true
		}
		c.r = c.riter()
	}
	return nil, false
}

func binopIterator(l Iter, r Iter, fn func(l, r interface{}) interface{}) Iter {
	return &binopIter{liter: reuseIterator(l), r: r, fn: fn}
}

type binopIter struct {
	liter func() Iter
	r     Iter
	fn    func(l, r interface{}) interface{}
	l     Iter
	rval  interface{}
	done  bool
}

func (c *binopIter) Next() (interface{}, bool) {
	for !c.done {
		if c.l != nil {
			if l, ok := c.l.Next(); ok {
				if l == struct{}{} {
					continue
				}
				if err, ok := l.(error); ok {
					c.done = true
					return err, true
				}
				return c.fn(l, c.rval), true
			}
			c.l = nil
		}
		r, ok := c.r.Next()
		if !ok {
			break
		}
		if err, ok := r.(error); ok {
			c.done = true
			return err, true
		}
		if r == struct{}{} {
			continue
		}
		c.rval, c.l = r, c.liter()
	}
	return nil, false
}

func reuseIterator(c Iter) func() Iter {
	xs, m := []interface{}{}, new(sync.Mutex)
	return func() Iter {
		return indexIterator(func(i int) (interface{}, bool) {
			m.Lock()
			defer m.Unlock()
			if i < len(xs) {
				return xs[i], true
			}
			for {
				v, ok := c.Next()
				if !ok {
					break
				}
				xs = append(xs, v)
				return v, true
			}
			return nil, false
		})
	}
}

func mapIterator(c Iter, f func(interface{}) interface{}) Iter {
	return mapIteratorWithError(c, func(v interface{}) interface{} {
		if _, ok := v.(error); ok {
			return v
		}
		return f(v)
	})
}

func mapIteratorWithError(c Iter, f func(interface{}) interface{}) Iter {
	return &mapWithErrorIter{src: c, f: f}
}

type mapWithErrorIter struct {
	src  Iter
	f    func(interface{}) interface{}
	iter Iter
	done bool
}

func (c *mapWithErrorIter) Next() (interface{}, bool) {
	for !c.done {
		if c.iter != nil {
			if v, ok := c.iter.Next(); ok {
				switch v.(type) {
				case struct{}:
					continue
				case *breakError:
					c.done = true
					return v, true
				default:
					return v, true
				}
			}
			c.iter = nil
		}
		v, ok := c.src.Next()
		if !ok {
			c.done = true
			return nil, false
		}
		x := c.f(v)
		if y, ok := x.(Iter); ok {
			c.iter = y
			continue
		}
		return x, true
	}
	return nil, false
}

func foldIterator(c Iter, x interface{}, f func(interface{}, interface{}) interface{}) Iter {
	return &foldIter{src: c, x: x, f: f}
}

type foldIter struct {
	src  Iter
	x    interface{}
	f    func(interface{}, interface{}) interface{}
	done bool
}

func (c *foldIter) Next() (interface{}, bool) {
	if c.done {
		return nil, false
	}
	for {
		v, ok := c.src.Next()
		if !ok {
			break
		}
		c.x = c.f(c.x, v)
		if _, ok := c.x.(error); ok {
			break
		}
	}
	c.done = true
	return c.x, true
}

func foreachIterator(c Iter, x interface{}, f func(interface{}, interface{}) (interface{}, Iter)) Iter {
	return &foreachIter{src: c, x: x, f: f}
}

type foreachIter struct {
	src  Iter
	x    interface{}
	f    func(interface{}, interface{}) (interface{}, Iter)
	iter Iter
	done bool
}

func (c *foreachIter) Next() (interface{}, bool) {
	for !c.done {
		if c.iter != nil {
			if v, ok := c.iter.Next(); ok {
				if v == struct{}{} {
					continue
				}
				if _, ok := v.(error); ok {
					c.done = true
					return nil, false
				}
				return v, true
			}
			c.iter = nil
		}
		v, ok := c.src.Next()
		if !ok {
			c.done = true
			break
		}
		c.x, c.iter = c.f(c.x, v)
	}
	return nil, false
}

func iteratorLast(c Iter) interface{} {
	var v interface{}
	for {
		w, ok := c.Next()
		if !ok {
			break
		}
		v = w
	}
	return v
}
