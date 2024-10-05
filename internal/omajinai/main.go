package main

import "context"

type Interface interface {
	Do(ctx context.Context) error
}

var _ Interface = (*Struct)(nil)

type Struct struct{}

// Do implements Interface.
func (str *Struct) Do(ctx context.Context) error {
	panic("unimplemented")
}
