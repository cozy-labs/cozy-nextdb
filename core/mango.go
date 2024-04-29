package core

import (
	"encoding/json"
	"fmt"
	"sort"
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
	sort.Strings(fields)
	parsed, err := parseMangoFields(fields)
	if err != nil {
		return "", err
	}
	return parsed.toSQL(""), nil
}

func parseMangoFields(fields []string) (mangoField, error) {
	var parsed mangoField
	for _, field := range fields {
		if field == "" || strings.ContainsRune(field, '\'') {
			return parsed, ErrBadRequest
		}
		addFieldToMangoFields(&parsed, field)
	}
	return parsed, nil
}

func addFieldToMangoFields(ptr *mangoField, field string) {
	parts := strings.Split(field, ".")
	for _, part := range parts {
		sub := findSubKey(ptr, part)
		if sub != nil {
			if len(sub.SubKeys) == 0 {
				return
			}
			ptr = sub
			continue
		}

		sub = &mangoField{Key: part}
		ptr.SubKeys = append(ptr.SubKeys, sub)
		ptr = sub
	}
}

func findSubKey(ptr *mangoField, key string) *mangoField {
	for _, sub := range ptr.SubKeys {
		if sub.Key == key {
			return sub
		}
	}
	return nil
}

type mangoField struct {
	Key     string
	SubKeys []*mangoField
}

func (f *mangoField) toSQL(path string) string {
	sql := "jsonb_build_object("
	for i, sub := range f.SubKeys {
		if i > 0 {
			sql += ", "
		}
		value := ""
		switch {
		case len(sub.SubKeys) > 0:
			value = sub.toSQL(path + sub.Key + ",")
		case path == "":
			value = fmt.Sprintf("blob -> '%s'", sub.Key)
		default:
			value = fmt.Sprintf("blob #> '{%s%s}'", path, sub.Key)
		}
		sql += fmt.Sprintf("'%s', %s", sub.Key, value)
	}
	sql += ")"
	return sql
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
		if strings.Contains(field, ".") {
			replaced := strings.ReplaceAll(field, ".", ",")
			orderBy += fmt.Sprintf("blob #> '{%s}' %s", replaced, way)
		} else {
			orderBy += fmt.Sprintf("blob -> '%s' %s", field, way)
		}
	}
	return orderBy, nil
}
