package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMangoFieldsToSQL(t *testing.T) {
	result, err := mangoFieldsToSQL(nil)
	assert.NoError(t, err)
	assert.Equal(t, result, "blob")

	result, err = mangoFieldsToSQL([]string{"one"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('one', blob ->> 'one')")

	result, err = mangoFieldsToSQL([]string{"one", "two", "three"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('one', blob ->> 'one', 'two', blob ->> 'two', 'three', blob ->> 'three')")

	_, err = mangoFieldsToSQL([]string{"SQL injection '; DROP TABLE ..."})
	assert.Error(t, err)
}

func TestMangoSortToSQL(t *testing.T) {
	result, err := mangoSortToSQL(nil)
	assert.NoError(t, err)
	assert.Equal(t, result, "row_id ASC")

	result, err = mangoSortToSQL([]any{"one", "two"})
	assert.NoError(t, err)
	assert.Equal(t, result, "blob ->> 'one' ASC, blob ->> 'two' ASC")

	result, err = mangoSortToSQL([]any{
		map[string]any{"one": "desc"},
		map[string]any{"two": "desc"},
	})
	assert.NoError(t, err)
	assert.Equal(t, result, "blob ->> 'one' DESC, blob ->> 'two' DESC")

	_, err = mangoSortToSQL([]any{1})
	assert.Error(t, err)

	_, err = mangoSortToSQL([]any{"SQL injection '; DROP TABLE..."})
	assert.Error(t, err)

	_, err = mangoSortToSQL([]any{
		map[string]any{"one": "invalid"},
	})
	assert.Error(t, err)

	_, err = mangoSortToSQL([]any{
		map[string]any{"one": "desc", "two": "desc"}, // invalid syntax
	})
	assert.Error(t, err)
}
