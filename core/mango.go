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
	Selector map[string]any
	Fields   []string
	Sort     []any
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
ORDER BY %s
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

	sort, err := mangoSortToSQL(params.Sort)
	if err != nil {
		return nil, err
	}

	response := &MangoResponse{}
	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		sql := fmt.Sprintf(FindMangoSQL, selected, table, sort)
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
		if field == "" || strings.ContainsRune(field, '\'') {
			return "", ErrBadRequest
		}
		if i > 0 {
			selected += ", "
		}
		selected += fmt.Sprintf("'%s', blob ->> '%s'", field, field)
		// TODO should we use ->> or -> ?
		// TODO nested fields like metadata.datetime
	}
	selected += ")"
	return selected, nil
}

func mangoSortToSQL(sort []any) (string, error) {
	if len(sort) == 0 {
		return "row_id ASC", nil
	}

	orderBy := ""
	for i, item := range sort {
		field := ""
		way := "ASC"
		switch item := item.(type) {
		case string:
			field = item
		case map[string]any:
			if len(item) != 1 {
				return "", ErrBadRequest
			}
			for k, v := range item {
				field = k
				w, ok := v.(string)
				if !ok {
					return "", ErrBadRequest
				}
				way = strings.ToUpper(w)
			}
		default:
			return "", ErrBadRequest
		}

		if field == "" || strings.ContainsRune(field, '\'') {
			return "", ErrBadRequest
		}
		if way != "ASC" && way != "DESC" {
			return "", ErrBadRequest
		}
		if i > 0 {
			orderBy += ", "
		}
		orderBy += fmt.Sprintf("blob ->> '%s' %s", field, way)
		// TODO should we use ->> or -> ?
		// TODO nested fields like metadata.datetime
	}
	return orderBy, nil
}
