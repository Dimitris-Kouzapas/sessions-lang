package util

import (
    "errors"
)

type Stack[T any] struct {
    data []T
}

// NewStack: nil slice is fine (zero alloc until first push)
func NewStack[T any]() *Stack[T] {
    return &Stack[T]{}
}

// Push adds an element to the top of the stack.
func (s *Stack[T]) Push(value T) {
	s.data = append(s.data, value)
}

// Pop removes and returns the top element of the stack.
// Returns an error if the stack is empty.
func (s *Stack[T]) Pop() (T, error) {
  	if len(s.data) == 0 {
    		var zeroValue T
    		return zeroValue, errors.New("stack is empty")
  	}
  	top := s.data[len(s.data)-1]
  	s.data = s.data[:len(s.data)-1]
  	return top, nil
}

// Peek returns the top element of the stack without removing it.
// Returns an error if the stack is empty.
func (s *Stack[T]) Top() (T, error) {
    if len(s.data) == 0 {
      	var zeroValue T
      	return zeroValue, errors.New("stack is empty")
    }
    return s.data[len(s.data)-1], nil
}

// IsEmpty checks if the stack is empty.
func (s *Stack[T]) IsEmpty() bool {
    return len(s.data) == 0
}

// Size returns the number of elements in the stack.
func (s *Stack[T]) Size() int {
    return len(s.data)
}
