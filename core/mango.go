package core

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type MangoParams struct {
	Fields   []string
	Selector map[string]any
	Limit    int
	Skip     int
}

type MangoResponse struct {
	Docs []json.RawMessage `json:"docs"`
	// TODO bookmark
}

const FindMangoSQL = `
SELECT %s
FROM %s
WHERE doctype = $1
AND kind = '` + string(NormalDocKind) + `'
ORDER BY row_id ASC
LIMIT $2
OFFSET $3
;
`

func (o *Operator) FindMango(databaseName string, params MangoParams) (*MangoResponse, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	limit := params.Limit
	if limit == 0 {
		limit = 25
	}

	selected, err := mangoFieldsToSQL(params.Fields)
	if err != nil {
		return nil, err
	}

	response := &MangoResponse{}
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		sql := fmt.Sprintf(FindMangoSQL, selected, table)
		sql = strings.ReplaceAll(sql, "\n", " ")
		rows, err := tx.Query(o.Ctx, sql, doctype, limit, params.Skip)
		if err != nil {
			return err
		}

		docs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (json.RawMessage, error) {
			var doc json.RawMessage
			err := row.Scan(&doc)
			return doc, err
		})
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			return err
		}

		response.Docs = docs
		return nil
	})
	return response, err
}

func mangoFieldsToSQL(fields []string) (string, error) {
	if len(fields) == 0 {
		return "blob", nil
	}

	selected := "jsonb_build_object("
	for i, field := range fields {
		if strings.ContainsRune(field, '\'') {
			return "", ErrBadRequest
		}
		if i > 0 {
			selected += ", "
		}
		selected += fmt.Sprintf("'%s', blob ->> '%s'", field, field)
	}
	selected += ")"
	return selected, nil
}
