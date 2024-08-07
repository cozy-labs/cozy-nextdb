package core

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
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

func ExtractGeneration(rev string) int {
	parts := strings.SplitN(rev, "-", 2)
	gen, err := strconv.Atoi(parts[0])
	if err != nil {
		return -1
	}
	return gen
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

	docID, _ := doc["_id"].(string)
	if docID == "" {
		docID = ShortUUID()
		doc["_id"] = docID
		body, err = json.Marshal(doc)
		if err != nil {
			return nil, err
		}
	}

	if _, ok := doc["_rev"]; ok {
		return nil, ErrConflict
	}

	if doc["_deleted"] == true {
		return o.doCreateDeletedDocument(table, doctype, docID)
	}

	revSum := ComputeRevisionSum(body)
	doc["_rev"] = "1-" + revSum

	return o.doCreateDocument(table, doctype, docID, revSum, doc)
}

func (o *Operator) doCreateDocument(table, doctype, docID, revSum string, doc map[string]any) (map[string]any, error) {
	err := o.ReadWriteTx(func(tx pgx.Tx) error {
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

		// TODO what if the document had been created, deleted, and we try again to recreate it?
		ok, err := o.ExecInsertRow(tx, table, doctype, NormalDocKind, docID, doc)
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
		ok, err = o.ExecInsertRow(tx, table, doctype, RevisionsKind, docID, blob)
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

func (o *Operator) doCreateDeletedDocument(table, doctype, docID string) (map[string]any, error) {
	doc := map[string]any{"_id": docID, "_deleted": true}
	body, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	revSum := ComputeRevisionSum(body)
	newRev := fmt.Sprintf("1-%s", revSum)
	doc["_rev"] = newRev

	err = o.ReadWriteTx(func(tx pgx.Tx) error {
		lastSeq, err := o.ExecIncrementLastSeq(tx, table, doctype)
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

		// TODO what if the document had been created, deleted, and we try again to recreate it?
		ok, err := o.ExecInsertRow(tx, table, doctype, NormalDocKind, docID, doc)
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
		ok, err = o.ExecInsertRow(tx, table, doctype, RevisionsKind, docID, blob)
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

		change := map[string]any{"id": doc["_id"], "rev": doc["_rev"], "deleted": true}
		body, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeSum := ComputeRevisionSum(body)
		changeID := fmt.Sprintf("%08x-%s", lastSeq, changeSum)
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

func (o *Operator) PutDocument(databaseName, docID, currentRev string, r io.Reader) (map[string]any, error) {
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

	bodyInvalidated := false
	if id, _ := doc["_id"].(string); id == "" {
		doc["_id"] = docID
		bodyInvalidated = true
	}

	rev := currentRev
	if _, hasRev := doc["_rev"]; hasRev {
		rev, _ = doc["_rev"].(string)
		if rev == "" {
			return nil, ErrConflict
		}
		if currentRev != "" && rev != currentRev {
			return nil, ErrConflict
		}
	} else if currentRev != "" {
		doc["_rev"] = currentRev
		bodyInvalidated = true
	}

	if doc["_deleted"] == true {
		if rev == "" {
			return o.doCreateDeletedDocument(table, doctype, docID)
		}
		return o.doDeleteDocument(table, doctype, docID, rev)
	}

	if bodyInvalidated {
		body, err = json.Marshal(doc)
		if err != nil {
			return nil, err
		}
	}

	gen := 0
	if rev != "" {
		gen = ExtractGeneration(rev)
		if gen <= 0 {
			return nil, ErrConflict
		}
	}
	revSum := ComputeRevisionSum(body)
	newRev := fmt.Sprintf("%d-%s", gen+1, revSum)
	doc["_rev"] = newRev

	if gen == 0 {
		return o.doCreateDocument(table, doctype, docID, revSum, doc)
	}
	return o.doUpdateDocument(table, doctype, docID, currentRev, revSum, doc)
}

func (o *Operator) doUpdateDocument(table, doctype, docID, currentRev, revSum string, doc map[string]any) (map[string]any, error) {
	err := o.ReadWriteTx(func(tx pgx.Tx) error {
		lastSeq, err := o.ExecIncrementLastSeq(tx, table, doctype)
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

		ok, err := o.ExecUpdateDocument(tx, table, doctype, NormalDocKind, docID, currentRev, doc)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return ErrConflict
				}
			}
			return err
		}
		if !ok {
			// We are making a request just to return the correct error message
			var result map[string]any
			err := o.ExecGetRow(tx, table, doctype, NormalDocKind, docID, &result)
			if err != nil {
				return ErrNotFound
			}
			return ErrConflict
		}

		var revisions RevsStruct
		err = o.ExecGetRow(tx, table, doctype, RevisionsKind, docID, &revisions)
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
		revisions.Start++
		revisions.IDs = prepend(revSum, revisions.IDs)
		ok, err = o.ExecUpdateRow(tx, table, doctype, RevisionsKind, docID, revisions)
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

		_, err = o.ExecDeleteChangeForDocument(tx, table, doctype, docID)
		if err != nil {
			return err
		}

		change := map[string]any{"id": doc["_id"], "rev": doc["_rev"]}
		body, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeSum := ComputeRevisionSum(body)
		changeID := fmt.Sprintf("%08x-%s", lastSeq, changeSum)
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

func prepend(item string, slice []string) []string {
	return append([]string{item}, slice...)
}

func (o *Operator) GetDocument(databaseName, docID string, withRevisions bool) (map[string]any, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	var result map[string]any
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		err = o.ExecGetRow(tx, table, doctype, NormalDocKind, docID, &result)
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
		if deleted, _ := result["_deleted"].(bool); deleted {
			return ErrDeleted
		}

		if withRevisions {
			var revisions map[string]any
			err = o.ExecGetRow(tx, table, doctype, RevisionsKind, docID, &revisions)
			if err != nil {
				return err
			}
			result["_revisions"] = revisions
		}

		return nil
	})
	return result, err
}

func (o *Operator) DeleteDocument(databaseName, docID, currentRev string) (map[string]any, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}
	return o.doDeleteDocument(table, doctype, docID, currentRev)
}

func (o *Operator) doDeleteDocument(table, doctype, docID, currentRev string) (map[string]any, error) {
	gen := ExtractGeneration(currentRev)
	if gen <= 0 {
		return nil, ErrConflict
	}

	doc := map[string]any{"_id": docID, "_rev": currentRev, "_deleted": true}
	body, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	revSum := ComputeRevisionSum(body)
	newRev := fmt.Sprintf("%d-%s", gen+1, revSum)
	doc["_rev"] = newRev

	err = o.ReadWriteTx(func(tx pgx.Tx) error {
		lastSeq, err := o.ExecDecrementDocCount(tx, table, doctype)
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

		ok, err := o.ExecUpdateDocument(tx, table, doctype, NormalDocKind, docID, currentRev, doc)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			return err
		}
		if !ok {
			// We are making a request just to return the correct error message
			var result map[string]any
			err := o.ExecGetRow(tx, table, doctype, NormalDocKind, docID, &result)
			if err != nil {
				return ErrNotFound
			}
			return ErrConflict
		}

		ok, err = o.ExecDeleteRow(tx, table, doctype, RevisionsKind, docID)
		if err != nil {
			return err
		}
		if !ok {
			return ErrInternalServerError
		}

		_, err = o.ExecDeleteChangeForDocument(tx, table, doctype, docID)
		if err != nil {
			return err
		}

		change := map[string]any{"id": doc["_id"], "rev": doc["_rev"], "deleted": true}
		body, err := json.Marshal(change)
		if err != nil {
			return err
		}
		changeSum := ComputeRevisionSum(body)
		changeID := fmt.Sprintf("%08x-%s", lastSeq, changeSum)
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
