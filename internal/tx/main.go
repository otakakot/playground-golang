package main

import "context"

type Tx interface {
	Tx(context.Context, func(context.Context) error) error
}

type ITx struct {
	db *DB
}

func (itx *ITx) Tx(ctx context.Context, fn func(context.Context) error) error {
	itx.db.Begin(ctx)

	if err := fn(ctx); err != nil {
		itx.db.Rollback(ctx)

		return err
	}

	return itx.db.Commit(ctx)
}

type DB struct {
}

func (db *DB) Begin(ctx context.Context) error {
	return nil
}

func (db *DB) Commit(ctx context.Context) error {
	return nil
}

func (db *DB) Rollback(ctx context.Context) error {
	return nil
}
