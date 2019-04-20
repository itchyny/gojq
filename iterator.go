package gojq

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

func reuseIterator(c <-chan interface{}) func() <-chan interface{} {
	var done bool
	var xs []interface{}
	return func() <-chan interface{} {
		d := make(chan interface{}, 1)
		if done {
			go func() {
				defer close(d)
				for _, v := range xs {
					d <- v
				}
			}()
		} else {
			done = true
			go func() {
				defer close(d)
				for v := range c {
					xs = append(xs, v)
					d <- v
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
