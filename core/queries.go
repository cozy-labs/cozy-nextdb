package core

import (
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type RowKind string

const (
	DoctypeKind   RowKind = "doctype"
	NormalDocKind RowKind = "normal_doc"
	DesignDocKind RowKind = "design_doc"
	LocalDocKind  RowKind = "local_doc"
	RevisionsKind RowKind = "revisions"
	ChangeKind    RowKind = "change"
)

const CreateDocumentKindSQL = `
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'row_kind') THEN
    CREATE TYPE row_kind AS ENUM (
      '` + string(DoctypeKind) + `',
      '` + string(NormalDocKind) + `',
      '` + string(DesignDocKind) + `',
      '` + string(LocalDocKind) + `',
      '` + string(RevisionsKind) + `',
      '` + string(ChangeKind) + `'
    );
  END IF;
END
$$ LANGUAGE plpgsql;
`

func (o *Operator) ExecCreateDocumentKind(tx pgx.Tx) (pgconn.CommandTag, error) {
	sql := strings.ReplaceAll(CreateDocumentKindSQL, "\n", " ") // easier to read in logs
	return tx.Exec(o.Ctx, sql)
}

const CreateTableSQL = `
CREATE TABLE %s (
  doctype VARCHAR(255),
  row_id  VARCHAR(255),
  kind    row_kind,
  blob    JSONB,
  PRIMARY KEY (doctype, kind, row_id)
);
`

func (o *Operator) ExecCreateTable(tx pgx.Tx, tableName string) (pgconn.CommandTag, error) {
	sql := fmt.Sprintf(CreateTableSQL, tableName)
	sql = strings.ReplaceAll(sql, "\n", " ")
	return tx.Exec(o.Ctx, sql)
}

const InsertRowSQL = `
INSERT INTO %s(doctype, row_id, kind, blob)
VALUES ($1, $2, '%s', $3);
`

func (o *Operator) ExecInsertRow(tx pgx.Tx, tableName, doctype string, kind RowKind, id string, blob any) (bool, error) {
	sql := fmt.Sprintf(InsertRowSQL, tableName, kind)
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, doctype, id, blob)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

const GetRowSQL = `
SELECT blob
FROM %s
WHERE doctype = $1
AND row_id = $2
AND kind = '%s';
`

func (o *Operator) ExecGetRow(tx pgx.Tx, tableName, doctype string, kind RowKind, id string, blob any) error {
	sql := fmt.Sprintf(GetRowSQL, tableName, kind)
	sql = strings.ReplaceAll(sql, "\n", " ")
	return tx.QueryRow(o.Ctx, sql, doctype, id).Scan(blob)
}

const UpdateDocumentSQL = `
UPDATE %s
SET blob = $1
WHERE kind = '%s'
AND doctype = $2
AND row_id = $3
AND blob ->> '_rev' = $4;
`

func (o *Operator) ExecUpdateDocument(tx pgx.Tx, tableName, doctype string, kind RowKind, docID, rev string, blob any) (bool, error) {
	sql := fmt.Sprintf(UpdateDocumentSQL, tableName, kind)
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, blob, doctype, docID, rev)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

const UpdateRowSQL = `
UPDATE %s
SET blob = $1
WHERE kind = '%s'
AND doctype = $2
AND row_id = $3
`

func (o *Operator) ExecUpdateRow(tx pgx.Tx, tableName, doctype string, kind RowKind, docID string, blob any) (bool, error) {
	sql := fmt.Sprintf(UpdateRowSQL, tableName, kind)
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, blob, doctype, docID)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

const DeleteRowSQL = `
DELETE FROM %s
WHERE doctype = $1
AND row_id = $2
AND kind = '%s';
`

func (o *Operator) ExecDeleteRow(tx pgx.Tx, tableName, doctype string, kind RowKind, id string) (bool, error) {
	sql := fmt.Sprintf(DeleteRowSQL, tableName, kind)
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, doctype, id)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

const GetAllDocsSQL = `
SELECT %s
FROM %s
WHERE doctype = $1
AND kind = '%s'
AND row_id >= $2
AND row_id <= $3
ORDER BY row_id %s
LIMIT %v
OFFSET $4
;
`

func (o *Operator) ExecGetAllDocs(tx pgx.Tx, tableName, doctype string, params AllDocsParams) ([]map[string]any, error) {
	fields := "jsonb_build_object('_id', blob ->> '_id', '_rev', blob ->> '_rev')"
	if params.IncludeDocs {
		fields = "blob"
	}
	var limit any = "All"
	if params.Limit > 0 {
		limit = params.Limit
	}
	from, to := params.StartKey, params.EndKey
	if to == "" {
		to = "\uffff"
	}
	order := "ASC"
	if params.Descending {
		order = "DESC"
		from, to = to, from
	}

	sql := fmt.Sprintf(GetAllDocsSQL, fields, tableName, NormalDocKind, order, limit)
	sql = strings.ReplaceAll(sql, "\n", " ")
	rows, err := tx.Query(o.Ctx, sql, doctype, from, to, params.Skip)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (map[string]any, error) {
		var doc map[string]any
		err := row.Scan(&doc)
		return doc, err
	})
}

const GetAllDoctypesSQL = `
SELECT doctype
FROM %s
WHERE kind = '` + string(DoctypeKind) + `'
ORDER BY doctype %s
LIMIT %v
OFFSET $1
;
`

func (o *Operator) ExecGetAllDoctypes(tx pgx.Tx, tableName string, params AllDocsParams) ([]string, error) {
	var limit any = "All"
	if params.Limit > 0 {
		limit = params.Limit
	}
	order := "ASC"
	if params.Descending {
		order = "DESC"
	}

	sql := fmt.Sprintf(GetAllDoctypesSQL, tableName, order, limit)
	sql = strings.ReplaceAll(sql, "\n", " ")
	rows, err := tx.Query(o.Ctx, sql, params.Skip)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[string])
}

const DeleteDoctypeSQL = `
DELETE FROM %s
WHERE doctype = $1
`

func (o *Operator) ExecDeleteDoctype(tx pgx.Tx, tableName, doctype string) (bool, error) {
	sql := fmt.Sprintf(DeleteDoctypeSQL, tableName)
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, doctype)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() > 0, nil
}

const CheckDoctypeExistsSQL = `
SELECT 1
FROM %s
WHERE doctype = $1
AND kind = 'doctype';
LIMIT 1
`

func (o *Operator) ExecCheckDoctypeExists(tx pgx.Tx, tableName, doctype string) (bool, error) {
	var nb int64
	sql := fmt.Sprintf(CheckDoctypeExistsSQL, tableName)
	sql = strings.ReplaceAll(sql, "\n", " ")
	err := tx.QueryRow(o.Ctx, sql, doctype).Scan(&nb)
	return nb > 0, err
}

const CheckTableIsEmptySQL = `
SELECT NOT EXISTS (SELECT 1 FROM %s) AS is_empty;
`

func (o *Operator) ExecCheckTableIsEmpty(tx pgx.Tx, tableName string) (bool, error) {
	var empty bool
	sql := fmt.Sprintf(CheckTableIsEmptySQL, tableName)
	sql = strings.ReplaceAll(sql, "\n", " ")
	err := tx.QueryRow(o.Ctx, sql).Scan(&empty)
	return empty, err
}

const DropTableSQL = `
DROP TABLE %s;
`

func (o *Operator) ExecDropTable(tx pgx.Tx, tableName string) error {
	sql := fmt.Sprintf(DropTableSQL, tableName)
	sql = strings.ReplaceAll(sql, "\n", " ")
	_, err := tx.Exec(o.Ctx, sql)
	return err
}

const IncrementDocCountSQL = `
UPDATE %s
SET blob = jsonb_set(blob, '{doc_count}', ((blob -> 'doc_count')::int %c 1)::text::jsonb)
WHERE kind = '` + string(DoctypeKind) + `'
AND row_id = $1
AND doctype = $1
`

func (o *Operator) ExecIncrementDocCount(tx pgx.Tx, tableName, doctype string) (bool, error) {
	sql := fmt.Sprintf(IncrementDocCountSQL, tableName, '+')
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, doctype)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

func (o *Operator) ExecDecrementDocCount(tx pgx.Tx, tableName, doctype string) (bool, error) {
	sql := fmt.Sprintf(IncrementDocCountSQL, tableName, '-')
	sql = strings.ReplaceAll(sql, "\n", " ")
	tag, err := tx.Exec(o.Ctx, sql, doctype)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}
