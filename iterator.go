package gojq

import "sync"

func unitIterator(v interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	defer func() {
		defer close(d)
		d <- v
	}()
	return d
}

func objectIterator(c <-chan interface{}, keys <-chan interface{}, values <-chan interface{}) <-chan interface{} {
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

func objectKeyIterator(c <-chan interface{}, keys <-chan interface{}, values <-chan interface{}) <-chan interface{} {
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

func stringIterator(xs []<-chan interface{}) <-chan interface{} {
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

func binopIteratorAlt(l <-chan interface{}, r <-chan interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		var done bool
		for v := range l {
			if _, ok := v.(error); ok {
				break
			}
			if valueToBool(v) {
				d <- v
				done = true
			}
		}
		if !done {
			for v := range r {
				d <- v
			}
		}
	}()
	return d
}

func binopIteratorOr(l <-chan interface{}, r <-chan interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		r := reuseIterator(r)
		for l := range l {
			if err, ok := l.(error); ok {
				d <- err
				return
			}
			if valueToBool(l) {
				d <- true
			} else {
				for r := range r() {
					if err, ok := r.(error); ok {
						d <- err
						return
					}
					d <- valueToBool(r)
				}
			}
		}
	}()
	return d
}

func binopIteratorAnd(l <-chan interface{}, r <-chan interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		r := reuseIterator(r)
		for l := range l {
			if err, ok := l.(error); ok {
				d <- err
				return
			}
			if valueToBool(l) {
				for r := range r() {
					if err, ok := r.(error); ok {
						d <- err
						return
					}
					d <- valueToBool(r)
				}
			} else {
				d <- false
			}
		}
	}()
	return d
}

func binopIterator(l <-chan interface{}, r <-chan interface{}, fn func(l, r interface{}) interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		l := reuseIterator(l)
		for r := range r {
			if err, ok := r.(error); ok {
				d <- err
				return
			}
			for l := range l() {
				if err, ok := l.(error); ok {
					d <- err
					return
				}
				d <- fn(l, r)
			}
		}
	}()
	return d
}

func reuseIterator(c <-chan interface{}) func() <-chan interface{} {
	xs, m := []interface{}{}, new(sync.Mutex)
	get := func(i int) (interface{}, bool) {
		m.Lock()
		defer m.Unlock()
		if i < len(xs) {
			return xs[i], false
		}
		for v := range c {
			xs = append(xs, v)
			return v, false
		}
		return nil, true
	}
	return func() <-chan interface{} {
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
		return d
	}
}

func mapIterator(c <-chan interface{}, f func(interface{}) interface{}) <-chan interface{} {
	return mapIteratorWithError(c, func(v interface{}) interface{} {
		if _, ok := v.(error); ok {
			return v
		}
		return f(v)
	})
}

func mapIteratorWithError(c <-chan interface{}, f func(interface{}) interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for v := range c {
			x := f(v)
			if y, ok := x.(<-chan interface{}); ok {
				for v := range y {
					if v == struct{}{} {
						continue
					}
					d <- v
				}
				continue
			}
			d <- x
		}
	}()
	return d
}

func foldIterator(c <-chan interface{}, x interface{}, f func(interface{}, interface{}) interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		for v := range c {
			x = f(x, v)
			if _, ok := x.(error); ok {
				break
			}
		}
		d <- x
	}()
	return d
}

func iteratorLast(c <-chan interface{}) interface{} {
	var v interface{}
	for v = range c {
	}
	return v
}
