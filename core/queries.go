package core

import (
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const CreateDocumentKindSQL = `
CREATE TYPE row_kind AS ENUM (
  'doctype',
  'normal_doc',
  'design_doc',
  'local_doc',
  'change'
);
`

const CreateTableSQL = `
CREATE TABLE %s (
  doctype VARCHAR(255),
  row_id  VARCHAR(255),
  kind    VARCHAR(255), -- TODO use row_kind
  blob    JSONB,
  PRIMARY KEY (doctype, kind, row_id)
);
`

func (o *Operator) ExecCreateTable(tx pgx.Tx, tableName string) (pgconn.CommandTag, error) {
	sql := fmt.Sprintf(CreateTableSQL, tableName)
	return tx.Exec(o.Ctx, sql)
}

const InsertDoctypeSQL = `
INSERT INTO %s(doctype, row_id, kind, blob)
VALUES ($1, $1, 'doctype', '{}'::jsonb);
`

func (o *Operator) ExecInsertDoctype(tx pgx.Tx, tableName, doctype string) (bool, error) {
	sql := fmt.Sprintf(InsertDoctypeSQL, tableName)
	tag, err := tx.Exec(o.Ctx, sql, doctype)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

const CheckDoctypeExistsSQL = `
SELECT COUNT(*)
FROM %s
WHERE doctype = $1
AND kind = 'doctype';
`

func (o *Operator) ExecCheckDoctypeExists(tx pgx.Tx, tableName, doctype string) (bool, error) {
	sql := fmt.Sprintf(CheckDoctypeExistsSQL, tableName)
	var nb int64
	err := tx.QueryRow(o.Ctx, sql, doctype).Scan(&nb)
	return nb > 0, err
}
