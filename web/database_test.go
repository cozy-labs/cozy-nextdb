package web

import (
	"context"
	"runtime/trace"
	"testing"
)

func TestDatabase(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, task := trace.NewTask(ctx, "TestDatabase")
	defer task.End()

	t.Run("Test the PUT /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, ctx)
		e.PUT("/açétone").Expect().Status(400).
			JSON().Object().HasValue("error", "illegal_database_name")
		e.PUT("/aBCD").Expect().Status(400).
			JSON().Object().HasValue("error", "illegal_database_name")
		e.PUT("/_foo").Expect().Status(400).
			JSON().Object().HasValue("error", "illegal_database_name")

		e.PUT("/twice").Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.PUT("/twice").Expect().Status(412).
			JSON().Object().HasValue("error", "file_exists")

		prefix := getPrefix("database")
		db1 := getDatabase(prefix, "doctype1")
		db2 := getDatabase(prefix, "doctype2")
		e.PUT("/{db}").WithPath("db", db1).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.PUT("/{db}").WithPath("db", db2).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
	})

	t.Run("Test the GET/HEAD /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, ctx)

		prefix := getPrefix("database")
		db := getDatabase(prefix, "doctype")
		e.PUT("/{db}").WithPath("db", db).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.HEAD("/{db}").WithPath("db", db).
			Expect().Status(200)
		e.GET("/{db}").WithPath("db", db).
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 0)

		e.GET("/{db}").WithPath("db", getDatabase(prefix, "no_such_doctype")).
			Expect().Status(404)
		e.GET("/{db}").WithPath("db", getDatabase("no_such_prefix", "doctype")).
			Expect().Status(404)
	})

	t.Run("Test the DELETE /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, ctx)
		e.PUT("/delete_me").Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.DELETE("/delete_me").Expect().Status(200).
			JSON().Object().HasValue("ok", true)
		e.DELETE("/delete_me").Expect().Status(404).
			JSON().Object().HasValue("error", "not_found")

		prefix := getPrefix("database")
		db1 := getDatabase(prefix, "doctype1")
		db2 := getDatabase(prefix, "doctype2")
		e.PUT("/{db}").WithPath("db", db1).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.PUT("/{db}").WithPath("db", db2).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.DELETE("/{db}").WithPath("db", db1).
			Expect().Status(200).
			JSON().Object().HasValue("ok", true)
		e.GET("/{db}").WithPath("db", db2).
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 0)
		e.DELETE("/{db}").WithPath("db", db2).
			Expect().Status(200).
			JSON().Object().HasValue("ok", true)
		e.GET("/{db}").WithPath("db", db2).
			Expect().Status(404)
	})

	t.Run("Test the GET /_all_dbs endpoint", func(t *testing.T) {
		e := launchTestServer(t, ctx)

		prefix := getPrefix("database")
		db1 := getDatabase(prefix, "doctype1")
		db2 := getDatabase(prefix, "doctype2")
		e.PUT("/{db}").WithPath("db", db1).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.PUT("/{db}").WithPath("db", db2).
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.GET("/_all_dbs").
			WithQuery("start_key", `"`+prefix+`/"`).
			WithQuery("end_key", `"`+prefix+`0"`).
			Expect().Status(200).
			JSON().Array().IsEqual([]string{prefix + "/doctype1", prefix + "/doctype2"})

		e.GET("/_all_dbs").Expect().Status(501)
	})
}
