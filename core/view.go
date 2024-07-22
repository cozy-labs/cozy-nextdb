package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/dop251/goja"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ViewResponse struct {
	Offset    int       `json:"offset"`
	TotalRows int       `json:"total_rows"`
	Rows      []ViewRow `json:"rows"`
}
type ViewRow struct {
	ID    string `json:"id"`
	Key   any    `json:"key"`
	Value any    `json:"value"`
}

const setupView = `
var isArray = Array.isArray
var _fn = %s
_fn(%s)
`

// https://docs.couchdb.org/en/stable/ddocs/views/intro.html#what-is-a-view
func mapView(jsFunc string, document map[string]any) ([][]any, error) {
	vm := goja.New()
	time.AfterFunc(100*time.Millisecond, func() {
		vm.Interrupt("halt")
	})

	var emitted [][]any
	err := vm.Set("emit", func(call goja.FunctionCall) goja.Value {
		args := make([]any, len(call.Arguments))
		for i, value := range call.Arguments {
			args[i] = value.Export()
		}
		emitted = append(emitted, args)
		return goja.Null()
	})
	if err != nil {
		return nil, err
	}

	doc, err := json.Marshal(document)
	if err != nil {
		return nil, err
	}
	js := fmt.Sprintf(setupView, jsFunc, doc)

	_, err = vm.RunString(js)
	if err != nil {
		return nil, err
	}
	return emitted, nil
}

func (o *Operator) GetView(databaseName, docID, viewName string) (*ViewResponse, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	response := &ViewResponse{}
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		var ddoc map[string]any
		err = o.ExecGetRow(tx, table, doctype, DesignDocKind, docID, &ddoc)
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
		views, ok := ddoc["views"].(map[string]any)
		if !ok {
			return ErrNotFound
		}
		view, ok := views[viewName].(map[string]any)
		if !ok {
			return ErrNotFound
		}
		jsFunc, ok := view["map"].(string)
		if !ok {
			return ErrNotFound
		}

		params := AllDocsParams{IncludeDocs: true}
		docs, err := o.ExecGetAllDocs(tx, table, doctype, params)
		if err != nil {
			return err
		}

		for _, doc := range docs {
			emitted, err := mapView(jsFunc, doc)
			if err != nil {
				return err
			}
			id, _ := doc["_id"].(string)
			row := ViewRow{
				ID:  id,
				Key: id,
			}
			if len(emitted) > 0 {
				row.Key = emitted[0]
				if len(emitted) > 1 {
					row.Value = emitted[1]
				}
			}
			response.Rows = append(response.Rows, row)
		}

		response.TotalRows = len(response.Rows)
		return nil
	})

	return response, err
}

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
