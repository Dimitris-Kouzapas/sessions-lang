package communication

import (
	"context"
	"sync/atomic"
)

type Channel[T any] interface {
	Activate()
	Deactivate()
	Send(T) bool
	Receive() (T, bool)
	Close()
}

type basicChannel[T any] struct {
 	ch chan T
 	active atomic.Bool
 	closed atomic.Bool
}

func NewBasicChannel[T any](active bool) *basicChannel[T] {
	return NewBasicChannelSize[T](active, 0)
}

func NewBasicChannelSize[T any](active bool, buf int) *basicChannel[T] {
	c := &basicChannel[T] {
		ch: make(chan T, buf),
	}
	c.active.Store(active)
	return c
}

func (c *basicChannel[T]) Activate()   { c.active.Store(true) }
func (c *basicChannel[T]) Deactivate() { c.active.Store(false) }

func (c *basicChannel[T]) Receive() (T, bool) {
	if !c.active.Load() {
		var zero T
		return zero, false	
	}
	value, ok := <- c.ch
	return value, ok
}

func (c *basicChannel[T]) Send(value T) bool {
	if !c.active.Load() || c.closed.Load() {
		return false	
	}
	c.ch <- value
	return true
}

func (c* basicChannel[T]) Close() {
	if c.closed.CompareAndSwap(false, true) {
		close(c.ch)
	}
}


// ReceiveCtx receives with cancellation or timeout via context.
func (c *basicChannel[T]) ReceiveCtx(ctx context.Context) (T, bool) {
	var zero T
	if !c.active.Load() {
		return zero, false
	}
	select {
		case <-ctx.Done():
			return zero, false
		case v, ok := <-c.ch:
			return v, ok
	}
}

// SendCtx sends with cancellation or timeout via context.
func (c *basicChannel[T]) SendCtx(ctx context.Context, v T) bool {
	if !c.active.Load() || c.closed.Load() {
		return false
	}
	select {
		case <-ctx.Done():
			return false
		case c.ch <- v:
			return true
	}
}
