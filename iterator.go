package gojq

func unitIterator(v interface{}) <-chan interface{} {
	c := make(chan interface{}, 1)
	defer func() {
		defer close(c)
		c <- v
	}()
	return c
}

func objectIterator(c <-chan interface{}, keys <-chan interface{}, values <-chan interface{}) <-chan interface{} {
	d := make(chan interface{}, 1)
	go func() {
		defer close(d)
		keys := reuseIterator(keys)
		values := reuseIterator(values)
		for m := range c {
			if err, ok := m.(error); ok {
				d <- err
				return
			}
			m := m.(map[string]interface{})
			for key := range keys() {
				if err, ok := key.(error); ok {
					d <- err
					return
				}
				var k string
				if l, ok := key.(string); ok {
					k = l
				} else {
					d <- &objectKeyNotStringError{key}
					return
				}
				for value := range values() {
					if err, ok := value.(error); ok {
						d <- err
						return
					}
					l := make(map[string]interface{})
					for k, v := range m {
						l[k] = v
					}
					l[k] = value
					d <- l
				}
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
