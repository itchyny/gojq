package gojq

type iterator struct {
	name string
	iter chan interface{}
}

func foldIterators(v interface{}, is []iterator) chan interface{} {
	c := unitIterator(v)
	for _, it := range is {
		c = productIterator(c, it.iter, it.name)
	}
	return c
}

func unitIterator(v interface{}) chan interface{} {
	c := make(chan interface{}, 1)
	defer func() {
		defer close(c)
		c <- v
	}()
	return c
}

func productIterator(c chan interface{}, t chan interface{}, name string) chan interface{} {
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
				if e == struct{}{} {
					continue
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

func reuseIterator(c chan interface{}) func() chan interface{} {
	var done bool
	var xs []interface{}
	return func() chan interface{} {
		d := make(chan interface{}, 1)
		if done {
			go func() {
				defer close(d)
				for _, e := range xs {
					d <- e
				}
			}()
		} else {
			go func() {
				defer func() {
					close(d)
					done = true
				}()
				for e := range c {
					xs = append(xs, e)
					d <- e
				}
			}()
		}
		return d
	}
}
