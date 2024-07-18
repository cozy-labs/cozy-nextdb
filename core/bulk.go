package core

import (
	"errors"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type AllDocsParams struct {
	IncludeDocs bool
	Descending  bool
	Limit       int
	Skip        int
	StartKey    string
	EndKey      string
}

type AllDocsResponse struct {
	Offset    int          `json:"offset"`
	TotalRows int          `json:"total_rows"`
	Rows      []AllDocsRow `json:"rows"`
}
type AllDocsRow struct {
	ID    string         `json:"id"`
	Key   string         `json:"key"`
	Value AllDocsValue   `json:"value"`
	Doc   map[string]any `json:"doc,omitempty"`
}
type AllDocsValue struct {
	Rev string `json:"rev"`
}

type JustDocCount struct {
	DocCount int `json:"doc_count"`
}

func (o *Operator) GetAllDocs(databaseName string, params AllDocsParams) (*AllDocsResponse, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	response := &AllDocsResponse{Offset: params.Skip}
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		var db JustDocCount
		err = o.ExecGetRow(tx, table, doctype, DoctypeKind, doctype, &db)
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
		response.TotalRows = db.DocCount
		if db.DocCount == 0 {
			return nil
		}

		// TODO we should also include design docs
		docs, err := o.ExecGetAllDocs(tx, table, doctype, params)
		if err != nil {
			return err
		}
		for _, doc := range docs {
			id, _ := doc["_id"].(string)
			rev, _ := doc["_rev"].(string)
			row := AllDocsRow{
				ID:    id,
				Key:   id,
				Value: AllDocsValue{Rev: rev},
			}
			if params.IncludeDocs {
				row.Doc = doc
			}
			response.Rows = append(response.Rows, row)
		}

		return nil
	})
	return response, err
}
