package core

import (
	"encoding/json"
	"io"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (o *Operator) CreateDocument(databaseName string, r io.Reader) (map[string]any, error) {
	doc := map[string]any{}
	if err := json.NewDecoder(r).Decode(&doc); err != nil {
		return nil, ErrBadRequest
	}

	id, ok := doc["_id"].(string)
	if !ok {
		uuidv7, err := uuid.NewV7()
		if err != nil {
			return nil, err
		}
		id = uuidv7.String()
		doc["_id"] = id
	}

	if _, ok := doc["_rev"]; ok {
		return nil, ErrConflict
	}
	rev := "1-123" // FIXME revision
	doc["_rev"] = rev

	table, doctype := ParseDatabaseName(databaseName)

	err := o.ReadWriteTx(func(tx pgx.Tx) error {
		ok, err := o.ExecIncrementDocCount(tx, table, doctype)
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

		ok, err = o.ExecInsertRow(tx, table, doctype, NormalDocKind, id, doc)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return ErrConflict
				}
			}
			return err
		}
		if !ok {
			return ErrInternalServerError
		}

		blob := []string{rev}
		ok, err = o.ExecInsertRow(tx, table, doctype, RevsListKind, id, blob)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return ErrConflict
				}
			}
			return err
		}
		if !ok {
			return ErrInternalServerError
		}

		// TODO insert row for the changes feed

		return nil
	})

	return doc, err
}
