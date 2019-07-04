package gojq

type stack struct {
	data  []block
	index int
	limit int
}

type block struct {
	value interface{}
	next  int
}

func newStack() *stack {
	return &stack{index: -1, limit: -1}
}

func (s *stack) push(v interface{}) {
	b := block{v, s.index}
	i := s.index + 1
	if i <= s.limit {
		i = s.limit + 1
	}
	s.index = i
	if i < len(s.data) {
		s.data[i] = b
	} else {
		s.data = append(s.data, b)
	}
}

func (s *stack) pop() interface{} {
	b := s.data[s.index]
	s.index = b.next
	return b.value
}

func (s *stack) save(f *fork) {
	if s.index >= s.limit {
		s.limit = s.index
	}
	f.index, f.limit = s.index, s.limit
}

func (s *stack) restore(f *fork) {
	s.index, s.limit = f.index, f.limit
}
