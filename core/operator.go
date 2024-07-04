package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
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
func ParseDatabaseName(databaseName string) (string, string, error) {
	unescaped, err := url.PathUnescape(databaseName)
	if err != nil {
		return "", "", ErrIllegalDatabaseName
	}
	if parts := strings.SplitN(unescaped, "/", 2); len(parts) == 2 {
		return parts[0], parts[1], nil
	}
	return "noprefix", databaseName, nil
}

func (o *Operator) GetDatabase(databaseName string) (map[string]any, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		err = o.ExecGetRow(tx, table, doctype, DoctypeKind, doctype, &result)
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
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return err
	}
	unescaped := fmt.Sprintf("%s/%s", table, doctype)
	if len(unescaped) == 0 || strings.ContainsFunc(unescaped, invalidCharForDatabaseName) {
		return ErrIllegalDatabaseName
	}
	if first := databaseName[0]; first < 'a' || first > 'z' {
		return ErrIllegalDatabaseName
	}
	blob := map[string]any{"doc_count": 0}

	// Happy path: we just insert the doctype
	insertRows := func(tx pgx.Tx) error {
		ok, err := o.ExecInsertRow(tx, table, doctype, DoctypeKind, doctype, blob)
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
	}
	err = o.ReadWriteTx(insertRows)
	if err == nil || err == ErrDatabaseExists {
		return err
	}

	// We may have to create the table before inserting the doctype, and
	// perhaps even if row_kind type. Errors are ignored for them, as we may
	// have two concurrent HTTP requests for creating databases for the same
	// Cozy, and only of one of them can create the type and the table.
	_ = o.ReadWriteTx(func(tx pgx.Tx) error {
		_, err := o.ExecCreateDocumentKind(tx)
		return err
	})
	_ = o.ReadWriteTx(func(tx pgx.Tx) error {
		_, err = o.ExecCreateTable(tx, table)
		return err
	})
	return o.ReadWriteTx(insertRows)
}

func (o *Operator) DeleteDatabase(databaseName string) error {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return err
	}

	return o.ReadWriteTx(func(tx pgx.Tx) error {
		ok, err := o.ExecDeleteDoctype(tx, table, doctype)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			return err
		}
		if !ok {
			return ErrNotFound
		}

		empty, err := o.ExecCheckTableIsEmpty(tx, table)
		if err != nil {
			return err
		}
		if empty {
			return o.ExecDropTable(tx, table)
		}
		return nil
	})
}

func (o *Operator) GetAllDatabases(params AllDocsParams) ([]string, error) {
	parts := strings.Split(params.StartKey, "/")
	table := parts[0]
	if table == "" {
		return nil, ErrNotImplemented
	}
	if !strings.HasPrefix(params.EndKey, table) {
		return nil, ErrNotImplemented
	}

	var dbs []string
	err := o.ReadWriteTx(func(tx pgx.Tx) error {
		doctypes, err := o.ExecGetAllDoctypes(tx, table, params)
		if err != nil {
			return err
		}
		for _, doctype := range doctypes {
			dbs = append(dbs, table+"/"+doctype)
		}
		return nil
	})
	return dbs, err
}
