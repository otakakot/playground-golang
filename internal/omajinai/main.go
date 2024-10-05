package main

import "context"

type Interface[T any] interface {
	Do(ctx context.Context) (*T, error)
}

var _ Interface[any] = (*Struct[any])(nil)

type Struct[T any] struct{}

// Do implements Interface.
func (s *Struct[T]) Do(ctx context.Context) (*T, error) {
	panic("unimplemented")
}
