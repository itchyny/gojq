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
			var k string
			if l, ok := key.(string); ok {
				k = l
			} else {
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
