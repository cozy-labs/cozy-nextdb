package core

import (
	"strings"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type ChangesParams struct {
	Limit int // Negative number means no limit
	Since string
}

type ChangesResponse struct {
	Results []map[string]any `json:"results"`
	LastSeq string           `json:"last_seq"`
	Pending int              `json:"pending"`
}

func (o *Operator) GetChanges(databaseName string, params ChangesParams) (*ChangesResponse, error) {
	table, doctype, err := ParseDatabaseName(databaseName)
	if err != nil {
		return nil, err
	}

	response := &ChangesResponse{LastSeq: "0", Pending: 0}
	if params.Since != "" {
		response.LastSeq = params.Since
	}

	err = o.ReadOnlyTx(func(tx pgx.Tx) error {
		params.Since = addPaddingToSeq(params.Since)
		rows, err := o.ExecGetChanges(tx, table, doctype, params)
		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok {
				if pgErr.Code == pgerrcode.UndefinedTable {
					return ErrNotFound
				}
			}
			return err
		}
		lastPaddedSeq := params.Since
		for _, row := range rows {
			result := changeToResult(row)
			response.Results = append(response.Results, result)
			lastPaddedSeq = row.Seq
		}

		if len(response.Results) > 0 {
			response.LastSeq = removePaddingFromSeq(lastPaddedSeq)
		}

		if params.Limit < 0 || len(response.Results) != params.Limit {
			return nil
		}

		pending, err := o.ExecCountPendingChanges(tx, table, doctype, lastPaddedSeq)
		if err != nil {
			return err
		}
		response.Pending = pending

		return nil
	})
	return response, err
}

// addPaddingToSeq adds some zeros to the start of the sequence string.
//
// In the web API, the seq parameter is like 42-abcdef, but in the database, it
// is padded with zeros at the start, like 00000042-abcdef, to allow sorting on
// strings to work according to the sequence number (99- comes before 100-).
func addPaddingToSeq(seq string) string {
	index := strings.Index(seq, "-")
	if index < 0 || index >= 8 {
		return seq
	}
	padding := strings.Repeat("0", 8-index)
	return padding + seq
}

func removePaddingFromSeq(seq string) string {
	for i := 0; i < 8; i++ {
		after, found := strings.CutPrefix(seq, "0")
		if !found {
			break
		}
		seq = after
	}
	return seq
}

func changeToResult(change changeRow) map[string]any {
	result := map[string]any{
		"id":  change.Blob["id"],
		"seq": removePaddingFromSeq(change.Seq),
		"changes": []any{
			map[string]any{"rev": change.Blob["rev"]},
		},
	}
	if deleted, ok := change.Blob["deleted"]; ok {
		result["deleted"] = deleted
	}
	return result
}
