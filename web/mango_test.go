package web

import (
	"context"
	"runtime/trace"
	"strings"
	"testing"
)

func TestMango(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, task := trace.NewTask(ctx, "TestMango")
	defer task.End()

	e := launchTestServer(t, ctx)
	prefix := getPrefix("mango")
	db := getDatabase(prefix, "doctype1")

	e.PUT("/{db}").WithPath("db", db).
		Expect().Status(201).
		JSON().Object().HasValue("ok", true)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "foo", "value": "foo", "note": 7, "optional": true}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "bar", "value": "bar", "note": 10, "optional": true}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "baz", "value": "baz", "note": 7}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "qux", "value": "qux", "note": 2, "nested": {"q": 1, "u": 1, "x": 1}}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "quux", "value": "quux", "note": 2, "nested": {"q": 1, "u": 2, "x": 1}}`)).
		Expect().Status(201)

	t.Run("Basic", func(t *testing.T) {
		e := launchTestServer(t, ctx)

		// Check errors
		e.POST("/{db}/_find").WithPath("db", db).
			WithBytes([]byte(`not_json`)).
			Expect().Status(400)
		e.POST("/{db}/_find").WithPath("db", getDatabase("no_such_prefix", "doctype")).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}}`)).
			Expect().Status(404)

		obj := e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}}`)).
			Expect().Status(200).
			JSON().Object()
		docs := obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"bar", "baz", "foo", "quux", "qux"} {
			doc := docs.Value(i).Object()
			doc.HasValue("_id", key)
			doc.Value("_rev").String().NotEmpty()
			doc.HasValue("value", key)
		}
	})

	t.Run("Fields", func(t *testing.T) {
		e := launchTestServer(t, ctx)

		obj := e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}, "fields": ["value"]}`)).
			Expect().Status(200).
			JSON().Object()
		docs := obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"bar", "baz", "foo", "quux", "qux"} {
			doc := docs.Value(i).Object()
			doc.NotContainsKey("_id")
			doc.NotContainsKey("_rev")
			doc.NotContainsKey("optional")
			doc.HasValue("value", key)
		}

		obj = e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}, "fields": ["_id", "_rev"]}`)).
			Expect().Status(200).
			JSON().Object()
		docs = obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"bar", "baz", "foo", "quux", "qux"} {
			doc := docs.Value(i).Object()
			doc.HasValue("_id", key)
			doc.Value("_rev").String().NotEmpty()
			doc.NotContainsKey("value")
		}

		obj = e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}, "fields": ["nested.u", "nested.x"]}`)).
			Expect().Status(200).
			JSON().Object()
		docs = obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"bar", "baz", "foo", "quux", "qux"} {
			doc := docs.Value(i).Object()
			doc.NotContainsKey("_id")
			doc.NotContainsKey("_rev")
			if key == "qux" || key == "quux" {
				nested := doc.Value("nested").Object()
				nested.NotContainsKey("q")
				nbU := len(strings.Split(key, "u")) - 1
				nested.HasValue("u", nbU)
				nested.HasValue("x", 1)
			}
		}
	})

	t.Run("Sort", func(t *testing.T) {
		e := launchTestServer(t, ctx)

		obj := e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}, "sort": ["note", "value"]}`)).
			Expect().Status(200).
			JSON().Object()
		docs := obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"quux", "qux", "baz", "foo", "bar"} {
			doc := docs.Value(i).Object()
			doc.HasValue("_id", key)
			doc.Value("_rev").String().NotEmpty()
			doc.HasValue("value", key)
		}

		obj = e.POST("/{db}/_find").WithPath("db", db).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"selector": {}, "sort": [{"note": "desc"}, {"nested.u": "desc"}, {"_id": "desc"}]}`)).
			Expect().Status(200).
			JSON().Object()
		docs = obj.Value("docs").Array()
		docs.Length().IsEqual(5)
		for i, key := range []string{"bar", "foo", "baz", "quux", "qux"} {
			doc := docs.Value(i).Object()
			doc.HasValue("_id", key)
			doc.Value("_rev").String().NotEmpty()
			doc.HasValue("value", key)
		}
	})

	// TODO test pagination (skip, limit, and bookmark)
}
