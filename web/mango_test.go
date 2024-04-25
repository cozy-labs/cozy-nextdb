package web

import (
	"context"
	"runtime/trace"
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
		WithBytes([]byte(`{"_id": "foo", "value": "foo"}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "bar", "value": "bar"}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "baz", "value": "baz"}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "qux", "value": "qux"}`)).
		Expect().Status(201)
	e.POST("/{db}").WithPath("db", db).
		WithHeader("Content-Type", "application/json").
		WithBytes([]byte(`{"_id": "quux", "value": "quux"}`)).
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

	// TODO test pagination (skip, limit, and bookmark)
}
