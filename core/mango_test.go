package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMangoFieldsToSQL(t *testing.T) {
	result, err := mangoFieldsToSQL(nil)
	assert.NoError(t, err)
	assert.Equal(t, result, "blob")

	result, err = mangoFieldsToSQL([]string{"one"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('one', blob -> 'one')")

	result, err = mangoFieldsToSQL([]string{"one", "two", "three"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('one', blob -> 'one', 'three', blob -> 'three', 'two', blob -> 'two')")

	result, err = mangoFieldsToSQL([]string{"nested.sub.subsub"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('nested', jsonb_build_object('sub', jsonb_build_object('subsub', blob #> '{nested,sub,subsub}')))")

	result, err = mangoFieldsToSQL([]string{"nested.sub.a", "nested.sub.b", "nested.c", "nested.c.d"})
	assert.NoError(t, err)
	assert.Equal(t, result, "jsonb_build_object('nested', jsonb_build_object('c', blob #> '{nested,c}', 'sub', jsonb_build_object('a', blob #> '{nested,sub,a}', 'b', blob #> '{nested,sub,b}')))")

	_, err = mangoFieldsToSQL([]string{"SQL injection '; DROP TABLE ..."})
	assert.Error(t, err)
}

func TestParseMangoFields(t *testing.T) {
	result, err := parseMangoFields([]string{"a", "b", "c"})
	require.NoError(t, err)
	require.Len(t, result.SubKeys, 3)
	for i := 0; i < 3; i++ {
		expected := fmt.Sprintf("%c", 'a'+i)
		assert.Equal(t, expected, result.SubKeys[i].Key)
		assert.Len(t, result.SubKeys[i].SubKeys, 0)
	}

	result, err = parseMangoFields([]string{"a", "a.b"})
	require.NoError(t, err)
	require.Len(t, result.SubKeys, 1)
	assert.Equal(t, "a", result.SubKeys[0].Key)
	assert.Len(t, result.SubKeys[0].SubKeys, 0)

	result, err = parseMangoFields([]string{"nested.sub.subsub", "nested.sub.xtra"})
	require.NoError(t, err)
	require.Len(t, result.SubKeys, 1)
	nested := result.SubKeys[0]
	assert.Equal(t, "nested", nested.Key)
	require.Len(t, nested.SubKeys, 1)
	sub := nested.SubKeys[0]
	assert.Equal(t, "sub", sub.Key)
	require.Len(t, sub.SubKeys, 2)
	subsub := sub.SubKeys[0]
	assert.Equal(t, "subsub", subsub.Key)
	assert.Len(t, subsub.SubKeys, 0)
	xtra := sub.SubKeys[1]
	assert.Equal(t, "xtra", xtra.Key)
	assert.Len(t, xtra.SubKeys, 0)
}

func TestMangoFieldToSQL(t *testing.T) {
	fields := &mangoField{SubKeys: []*mangoField{
		{Key: "a"},
		{Key: "b"},
		{Key: "c"},
	}}
	result := fields.toSQL("")
	assert.Equal(t, result, "jsonb_build_object('a', blob -> 'a', 'b', blob -> 'b', 'c', blob -> 'c')")

	fields = &mangoField{SubKeys: []*mangoField{
		{Key: "nested", SubKeys: []*mangoField{
			{Key: "sub", SubKeys: []*mangoField{
				{Key: "subsub"},
			}},
		}},
	}}
	result = fields.toSQL("")
	assert.Equal(t, result, "jsonb_build_object('nested', jsonb_build_object('sub', jsonb_build_object('subsub', blob #> '{nested,sub,subsub}')))")
}

func TestMangoSortToSQL(t *testing.T) {
	result, err := mangoSortToSQL(nil)
	assert.NoError(t, err)
	assert.Equal(t, result, "row_id ASC")

	result, err = mangoSortToSQL([]any{"one", "two"})
	assert.NoError(t, err)
	assert.Equal(t, result, "blob -> 'one' ASC, blob -> 'two' ASC")

	result, err = mangoSortToSQL([]any{
		map[string]any{"one": "desc"},
		map[string]any{"two": "desc"},
	})
	assert.NoError(t, err)
	assert.Equal(t, result, "blob -> 'one' DESC, blob -> 'two' DESC")

	result, err = mangoSortToSQL([]any{"nested.sub.subsub"})
	assert.NoError(t, err)
	assert.Equal(t, result, "blob #> '{nested,sub,subsub}' ASC")

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
