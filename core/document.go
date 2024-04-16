package core

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gofrs/uuid/v5"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func ShortUUID() string {
	uuidv7, err := uuid.NewV7()
	if err != nil {
		panic(err)
	}
	return strings.ReplaceAll(uuidv7.String(), "-", "")
}

func ComputeRevisionSum(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[0:16])
}

type RevsStruct struct {
	Start int      `json:"start"`
	IDs   []string `json:"ids"`
}

func (o *Operator) CreateDocument(databaseName string, r io.Reader) (map[string]any, error) {
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

	id, ok := doc["_id"].(string)
	if !ok {
		id = ShortUUID()
		doc["_id"] = id
	}

	if _, ok := doc["_rev"]; ok {
		return nil, ErrConflict
	}
	revSum := ComputeRevisionSum(body)
	doc["_rev"] = "1-" + revSum

	err = o.ReadWriteTx(func(tx pgx.Tx) error {
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

		blob := RevsStruct{
			Start: 1,
			IDs:   []string{revSum},
		}
		ok, err = o.ExecInsertRow(tx, table, doctype, RevisionsKind, id, blob)
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

func (o *Operator) GetDocument(databaseName, docID string, withRevisions bool) (map[string]any, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		res, err := o.ExecGetRow(tx, table, doctype, NormalDocKind, docID)
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

		if withRevisions {
			revisions, err := o.ExecGetRow(tx, table, doctype, RevisionsKind, docID)
			if err != nil {
				return err
			}
			result["_revisions"] = revisions
		}

		return nil
	})
	return result, err
}
