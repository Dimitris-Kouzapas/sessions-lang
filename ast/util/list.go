package util

import (
	"fmt"
)

type ListElementI interface {
	// comparable
	fmt.Stringer
}

type List[T ListElementI] interface {
	Len() int
	Strings() []string
	String(i int) string
	Contains(T) bool
	Iterate(func(int, T))
	Iterator() func() (int, T, bool)
}

type listImpl[T ListElementI] struct {
	list []T
	internal Set[string]
}

func NewList[T ListElementI](list []T) *listImpl[T] {
	internal := NewHashSet[string]()
	for _, element := range list {
		internal.Add(element.String())
	}
	return &listImpl[T] {
		list: list,
		internal: internal,
	}
}

func mapList[U any, T ListElementI](f func(T) U, l List[T]) []U {
	u := make([]U, l.Len())
	iterator := l.Iterator()
	for i, element, hasElement := iterator(); hasElement; i, element, hasElement = iterator() {
		u[i] = f(element)
	}
	return u
}

func Map[U ListElementI, T ListElementI](f func(T) U, l List[T]) List[U] {
	u := mapList(f, l)
	return NewList[U](u)
}

func (l *listImpl[T]) Len() int {
	return len(l.list)
}

func (l *listImpl[T]) Strings() []string {
	f := func(element T) string { return element.String() }
	return mapList(f, l)
}

func (l *listImpl[T]) String(i int) string {
	return l.list[i].String()
}

func (l *listImpl[T]) Contains(element T) bool {
	return l.internal.Contains(element.String())
}

func (l *listImpl[T]) Iterate(f func(int, T)) {
	for i, element := range l.list {
		f(i, element)
	}
}

func (l *listImpl[T]) Iterator() func() (int, T, bool) {
	i := 0
	return func() (int, T, bool) {
		if i < len(l.list) {
			i++
			return i - 1, l.list[i-1], true
		}
		var element T
		return i, element, false
	}
}
