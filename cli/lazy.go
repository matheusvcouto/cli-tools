package cli

import "sync"

// Lazy initializes a dependency at most once. It is intended for expensive or
// stateful services that static CLI paths (help/version/schema/completion of
// static values) must not initialize.
type Lazy[T any] struct {
	once  sync.Once
	init  func() (T, error)
	value T
	err   error
}

func NewLazy[T any](init func() (T, error)) *Lazy[T] {
	return &Lazy[T]{init: init}
}

func (l *Lazy[T]) Get() (T, error) {
	l.once.Do(func() {
		if l.init == nil {
			var zero T
			l.value = zero
			return
		}
		l.value, l.err = l.init()
	})
	return l.value, l.err
}
