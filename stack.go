package gojq

type stack[T any] struct {
	data  []block[T]
	index int
	limit int
}

type block[T any] struct {
	value T
	next  int
}

func newStack[T any]() *stack[T] {
	return &stack[T]{index: -1, limit: -1}
}

func (s *stack[T]) push(v T) {
	b := block[T]{v, s.index}
	s.index = max(s.index, s.limit) + 1
	if s.index < len(s.data) {
		s.data[s.index] = b
	} else {
		s.data = append(s.data, b)
	}
}

func (s *stack[T]) pop() T {
	b := s.data[s.index]
	s.index = b.next
	return b.value
}

func (s *stack[T]) top() T {
	return s.data[s.index].value
}

func (s *stack[T]) empty() bool {
	return s.index < 0
}

func (s *stack[T]) save() (index, limit int) {
	index, limit = s.index, s.limit
	if s.index > s.limit {
		s.limit = s.index
	}
	return
}

func (s *stack[T]) restore(index, limit int) {
	s.index, s.limit = index, limit
}
