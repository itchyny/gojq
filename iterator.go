package gojq

type iterator struct {
	name string
	iter <-chan interface{}
}

func foldIterators(is []iterator) <-chan interface{} {
	c := unitIterator(map[string]interface{}{})
	for _, it := range is {
		c = productIterator(c, it.iter, it.name)
	}
	return c
}

func unitIterator(v interface{}) <-chan interface{} {
	c := make(chan interface{}, 1)
	defer func() {
		defer close(c)
		c <- v
	}()
	return c
}

func productIterator(c <-chan interface{}, t <-chan interface{}, name string) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		t := reuseIterator(t)
		for m := range c {
			if err, ok := m.(error); ok {
				d <- err
				return
			}
			m := m.(map[string]interface{})
			for e := range t() {
				if err, ok := e.(error); ok {
					d <- err
					return
				}
				n := make(map[string]interface{})
				for k, v := range m {
					n[k] = v
				}
				n[name] = e
				d <- n
			}
		}
	}()
	return d
}

func reuseIterator(c <-chan interface{}) func() <-chan interface{} {
	var done bool
	var xs []interface{}
	return func() <-chan interface{} {
		d := make(chan interface{}, 1)
		if done {
			go func() {
				defer close(d)
				for _, e := range xs {
					d <- e
				}
			}()
		} else {
			done = true
			go func() {
				defer close(d)
				for e := range c {
					xs = append(xs, e)
					d <- e
				}
			}()
		}
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
		for e := range c {
			x := f(e)
			if y, ok := x.(<-chan interface{}); ok {
				for e := range y {
					if e == struct{}{} {
						continue
					}
					d <- e
				}
				continue
			}
			d <- x
		}
	}()
	return d
}
