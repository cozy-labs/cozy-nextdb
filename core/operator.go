package core

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Operator struct {
	PG     *pgxpool.Pool
	Logger *slog.Logger
	Ctx    context.Context
}

func (o *Operator) Ping() error {
	return o.PG.Ping(o.Ctx)
}

// ReadWriteTx begins a transaction in read-write mode.
func (o *Operator) ReadWriteTx(fn func(pgx.Tx) error) error {
	return beginTx(o, pgx.ReadWrite, fn)
}

// ReadOnlyTx begins a transaction in read-only mode.
func (o *Operator) ReadOnlyTx(fn func(pgx.Tx) error) error {
	return beginTx(o, pgx.ReadOnly, fn)
}

func beginTx(o *Operator, accessMode pgx.TxAccessMode, fn func(pgx.Tx) error) error {
	opts := pgx.TxOptions{
		IsoLevel:   pgx.ReadCommitted,
		AccessMode: accessMode,
	}
	return pgx.BeginTxFunc(o.Ctx, o.PG, opts, fn)
}
