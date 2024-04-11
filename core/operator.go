package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

// ParseDatabaseName takes a database name (as in the CouchDB API), and returns
// the SQL table name and doctype for it.
func ParseDatabaseName(databaseName string) (string, string) {
	if parts := strings.SplitN(databaseName, "/", 2); len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "noprefix", databaseName
}

func (o *Operator) GetDatabase(databaseName string) (any, error) {
	var result any
	table, doctype := ParseDatabaseName(databaseName)
	err := o.ReadOnlyTx(func(tx pgx.Tx) error {
		res, err := o.ExecGetDoctype(tx, table, doctype)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			return err
		}
		result = res
		return nil
	})
	return result, err
}

func invalidCharForDatabaseName(r rune) bool {
	if 'a' <= r && r <= 'z' {
		return false
	}
	if '0' <= r && r <= '9' {
		return false
	}
	switch r {
	case '_', '$', '(', ')', '+', '-', '/':
		return false
	}
	return true
}

func (o *Operator) CreateDatabase(databaseName string) error {
	if len(databaseName) == 0 || strings.ContainsFunc(databaseName, invalidCharForDatabaseName) {
		return ErrIllegalDatabaseName
	}
	if first := databaseName[0]; first < 'a' || first > 'z' {
		return ErrIllegalDatabaseName
	}
	table, doctype := ParseDatabaseName(databaseName)

	// Happy path: we just insert the doctype
	err := o.ReadWriteTx(func(tx pgx.Tx) error {
		ok, err := o.ExecInsertDoctype(tx, table, doctype)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return ErrDatabaseExists
				}
			}
			return err
		}
		if !ok {
			return ErrInternalServerError
		}
		return nil
	})
	if err == nil || err == ErrDatabaseExists {
		return err
	}

	// Maybe we have to create the table before inserting the doctype
	return o.ReadWriteTx(func(tx pgx.Tx) error {
		if _, err := o.ExecCreateDocumentKind(tx); err != nil {
			return err
		}
		if _, err = o.ExecCreateTable(tx, table); err != nil {
			return err
		}
		ok, err := o.ExecInsertDoctype(tx, table, doctype)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return ErrDatabaseExists
				}
			}
			return err
		}
		if !ok {
			return ErrInternalServerError
		}
		return nil
	})
}
