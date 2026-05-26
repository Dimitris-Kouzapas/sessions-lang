package util

type Set[T comparable] interface {
    Add(T)
    AddList([]T)
    Remove(T)
    Contains(T) bool
    Size() int
    Clear()
    Slice() []T
}

type hashSet[T comparable] struct {
    data map[T]struct{}
}

// NewHashSet creates a new empty hashSet.
func NewHashSet[T comparable]() *hashSet[T] {
    return &hashSet[T]{
        data: make(map[T]struct{}),
    }
}

// Add inserts an element into the hashSet.
func (s *hashSet[T]) Add(value T) {
    s.data[value] = struct{}{}
}

func (s *hashSet[T]) AddList(list []T) {
    for _, value := range list {
        s.Add(value)
    }
}

// Remove deletes an element from the hashSet.
func (s *hashSet[T]) Remove(value T) {
    delete(s.data, value)
}

// Contains checks if an element exists in the hashSet.
func (s *hashSet[T]) Contains(value T) bool {
    _, exists := s.data[value]
    return exists
}

// Size returns the number of elements in the hashSet.
func (s *hashSet[T]) Size() int {
    return len(s.data)
}

// Clear removes all elements from the hashSet.
func (s *hashSet[T]) Clear() {
    s.data = make(map[T]struct{})
}

// Slice returns a slice of all elements in the hashSet.
func (s *hashSet[T]) Slice() []T {
    keys := make([]T, 0, len(s.data))
    for key := range s.data {
        keys = append(keys, key)
    }
    return keys
}
