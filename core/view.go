package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func (o *Operator) CreateDesignDoc(databaseName, docID string, r io.Reader) (map[string]any, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	body, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	doc := map[string]any{}
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, ErrBadRequest
	}

	if _, ok := doc["_rev"]; ok {
		return nil, ErrConflict
	}

	revSum := ComputeRevisionSum(body)
	doc["_rev"] = "1-" + revSum

	err = o.ReadWriteTx(func(tx pgx.Tx) error {
		lastSeq, err := o.ExecIncrementDocCount(tx, table, doctype)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return ErrNotFound
			}
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			return err
		}

		// TODO what if the ddoc had been created, deleted, and we try again to recreate it?
		ok, err := o.ExecInsertRow(tx, table, doctype, DesignDocKind, docID, doc)
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

		change := map[string]any{"id": doc["_id"], "rev": doc["_rev"]}
		body, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeSum := ComputeRevisionSum(body)
		changeID := fmt.Sprintf("%08d-%s", lastSeq, changeSum)
		ok, err = o.ExecInsertRow(tx, table, doctype, ChangeKind, changeID, change)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInternalServerError
		}

		return nil
	})

	return doc, err
}
