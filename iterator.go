package gojq

import "sync"

// Iter ...
type Iter interface {
	Next() (interface{}, bool)
}

func chanIterator(c <-chan interface{}) Iter {
	return &chanIter{c}
}

type chanIter struct {
	c <-chan interface{}
}

func (t *chanIter) Next() (interface{}, bool) {
	v, ok := <-t.c
	return v, ok
}

func unitIterator(v interface{}) Iter {
	d := make(chan interface{}, 1)
	defer func() {
		defer close(d)
		d <- v
	}()
	return chanIterator(d)
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
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		var done bool
		for {
			v, ok := l.Next()
			if !ok {
				break
			}
			if _, ok := v.(error); ok {
				d <- v
				done = true
				break
			}
			if v == struct{}{} {
				continue
			}
			if valueToBool(v) {
				d <- v
				done = true
			}
		}
		if !done {
			for {
				v, ok := r.Next()
				if !ok {
					break
				}
				if v == struct{}{} {
					continue
				}
				d <- v
			}
		}
	}()
	return chanIterator(d)
}

func binopIteratorOr(l Iter, r Iter) Iter {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		r := reuseIterator(r)
		for {
			l, ok := l.Next()
			if !ok {
				break
			}
			if err, ok := l.(error); ok {
				d <- err
				return
			}
			if l == struct{}{} {
				continue
			}
			if valueToBool(l) {
				d <- true
			} else {
				iter := r()
				for {
					r, ok := iter.Next()
					if !ok {
						break
					}
					if err, ok := r.(error); ok {
						d <- err
						return
					}
					if r == struct{}{} {
						continue
					}
					d <- valueToBool(r)
				}
			}
		}
	}()
	return chanIterator(d)
}

func binopIteratorAnd(l Iter, r Iter) Iter {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		r := reuseIterator(r)
		for {
			l, ok := l.Next()
			if !ok {
				break
			}
			if err, ok := l.(error); ok {
				d <- err
				return
			}
			if l == struct{}{} {
				continue
			}
			if valueToBool(l) {
				iter := r()
				for {
					r, ok := iter.Next()
					if !ok {
						break
					}
					if err, ok := r.(error); ok {
						d <- err
						return
					}
					if r == struct{}{} {
						continue
					}
					d <- valueToBool(r)
				}
			} else {
				d <- false
			}
		}
	}()
	return chanIterator(d)
}

func binopIterator(l Iter, r Iter, fn func(l, r interface{}) interface{}) Iter {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		l := reuseIterator(l)
		for {
			r, ok := r.Next()
			if !ok {
				break
			}
			if err, ok := r.(error); ok {
				d <- err
				return
			}
			if r == struct{}{} {
				continue
			}
			iter := l()
			for {
				l, ok := iter.Next()
				if !ok {
					break
				}
				if err, ok := l.(error); ok {
					d <- err
					return
				}
				if l == struct{}{} {
					continue
				}
				d <- fn(l, r)
			}
		}
	}()
	return chanIterator(d)
}

func reuseIterator(c Iter) func() Iter {
	xs, m := []interface{}{}, new(sync.Mutex)
	get := func(i int) (interface{}, bool) {
		m.Lock()
		defer m.Unlock()
		if i < len(xs) {
			return xs[i], false
		}
		for {
			v, ok := c.Next()
			if !ok {
				break
			}
			xs = append(xs, v)
			return v, false
		}
		return nil, true
	}
	return func() Iter {
		d := make(chan interface{}, 1)
		go func() {
			defer close(d)
			var i int
			for {
				v, done := get(i)
				if done {
					return
				}
				d <- v
				i++
			}
		}()
		return chanIterator(d)
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
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for {
			v, ok := c.Next()
			if !ok {
				break
			}
			x := f(v)
			if y, ok := x.(Iter); ok {
				for {
					v, ok := y.Next()
					if !ok {
						break
					}
					if v == struct{}{} {
						continue
					} else if e, ok := v.(*breakError); ok {
						d <- e
						return
					}
					d <- v
				}
				continue
			} else if e, ok := x.(*breakError); ok {
				d <- e
				return
			}
			d <- x
		}
	}()
	return chanIterator(d)
}

func foldIterator(c Iter, x interface{}, f func(interface{}, interface{}) interface{}) Iter {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for {
			v, ok := c.Next()
			if !ok {
				break
			}
			x = f(x, v)
			if _, ok := x.(error); ok {
				break
			}
		}
		d <- x
	}()
	return chanIterator(d)
}

func foreachIterator(c Iter, x interface{}, f func(interface{}, interface{}) (interface{}, Iter)) Iter {
	d := make(chan interface{}, 1)
	go func() {
		var y Iter
		defer close(d)
		for {
			v, ok := c.Next()
			if !ok {
				break
			}
			x, y = f(x, v)
			for {
				v, ok := y.Next()
				if !ok {
					break
				}
				if v == struct{}{} {
					continue
				}
				d <- v
			}
			if _, ok := x.(error); ok {
				break
			}
		}
	}()
	return chanIterator(d)
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
