package web

import (
	"strings"
	"testing"
)

func TestDoc(t *testing.T) {
	t.Parallel()
	container := preparePG(t)
	logger := setupLogger(t)

	t.Run("Test the POST /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, logger, connectToPG(t, container, logger))
		e.PUT("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)

		// Check errors
		e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithBytes([]byte(`not_json`)).
			Expect().Status(400)
		e.POST("/{db}").WithPath("db", "cozy0%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such cozy"}`)).
			Expect().Status(404)
		e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype2").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such doctype"}`)).
			Expect().Status(404)
		e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "foo", "_rev": "2-345"}`)).
			Expect().Status(409)

		// With a generated id
		obj := e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.Value("id").String().NotEmpty().Length().IsEqual(32)
		obj.Value("rev").String().NotEmpty().HasPrefix("1-").
			Length().IsEqual(34) // 2 bytes for 1- and 32 bytes for the checksum

		// With a named id
		obj = e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "myid"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "myid")
		obj.Value("rev").String().NotEmpty().HasPrefix("1-")
		e.GET("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 2)

		// CouchDB allows to create a deleted doc (sic)
		obj = e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_deleted": true}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.Value("id").String().NotEmpty().Length().IsEqual(32)
		obj.Value("rev").String().NotEmpty().HasPrefix("1-")
		e.GET("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 2)
	})

	t.Run("Test the GET /:db/:docid endpoint", func(t *testing.T) {
		e := launchTestServer(t, logger, connectToPG(t, container, logger))
		e.PUT("/{db}").WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		obj := e.POST("/{db}").WithPath("db", "cozy2%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		id := obj.Value("id").String().Raw()
		rev := obj.Value("rev").String().Raw()

		// Check errors
		e.HEAD("/{db}/nosuchid").WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(404)
		e.GET("/{db}/nosuchid").WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(404)

		// Test HEAD
		e.HEAD("/{db}/"+id).WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(200).
			Header("ETag").IsEqual(rev)
		obj = e.GET("/{db}/"+id).WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("_id", id)
		obj.HasValue("_rev", rev)
		obj.HasValue("foo", "bar")

		// Test GET
		obj = e.GET("/{db}/"+id).WithPath("db", "cozy2%2Fdoctype1").
			WithQuery("revs", "true").
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("_id", id)
		obj.HasValue("_rev", rev)
		obj.HasValue("foo", "bar")
		revs := obj.Value("_revisions").Object()
		revs.HasValue("start", 1)
		revs.HasValue("ids", []string{strings.TrimPrefix(rev, "1-")})
	})

	t.Run("Test the DELETE /:db/:docid endpoint", func(t *testing.T) {
		e := launchTestServer(t, logger, connectToPG(t, container, logger))
		e.PUT("/{db}").WithPath("db", "cozy3%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		obj := e.POST("/{db}").WithPath("db", "cozy3%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		id := obj.Value("id").String().Raw()
		rev := obj.Value("rev").String().Raw()

		// Check errors
		e.DELETE("/{db}/"+id).WithPath("db", "cozy3%2Fdoctype1").
			WithQuery("rev", "1-bad").
			Expect().Status(409).
			JSON().Object()

		// Good case
		obj = e.DELETE("/{db}/"+id).WithPath("db", "cozy3%2Fdoctype1").
			WithQuery("rev", rev).
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", id)
		obj.Value("rev").String().NotEmpty().NotEqual(rev).HasPrefix("2-")

		e.HEAD("/{db}/"+id).WithPath("db", "cozy3%2Fdoctype1").
			Expect().Status(404)
		e.GET("/{db}").WithPath("db", "cozy3%2Fdoctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 0)
	})

	t.Run("Test the PUT /:db/:doctype endpoint", func(t *testing.T) {
		e := launchTestServer(t, logger, connectToPG(t, container, logger))
		e.PUT("/{db}").WithPath("db", "cozy4%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)

		// Check errors
		e.PUT("/{db}/invalid1").WithPath("db", "cozy4%2Fdoctype1").
			WithBytes([]byte(`not_json`)).
			Expect().Status(400)
		e.PUT("/{db}/invalid2").WithPath("db", "cozy0%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such cozy"}`)).
			Expect().Status(404)
		e.PUT("/{db}/invalid3").WithPath("db", "cozy4%2Fdoctype2").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such doctype"}`)).
			Expect().Status(404)

		// Create a new document
		obj := e.PUT("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "doc1")
		rev1 := obj.Value("rev").String().NotEmpty().HasPrefix("1-").Raw()

		obj = e.GET("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("_id", "doc1")
		obj.HasValue("_rev", rev1)
		obj.HasValue("foo", "bar")

		// Update a document
		obj = e.PUT("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			WithQuery("rev", rev1).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "baz"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "doc1")
		rev2 := obj.Value("rev").String().NotEmpty().HasPrefix("2-").Raw()

		obj = e.GET("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("_id", "doc1")
		obj.HasValue("_rev", rev2)
		obj.HasValue("foo", "baz")

		e.GET("/{db}").WithPath("db", "cozy4%2Fdoctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 1)

		// Revisions must match
		e.PUT("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			WithQuery("rev", rev1).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"courge": "qux"}`)).
			Expect().Status(409)
		e.PUT("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			WithQuery("rev", rev2).
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "doc1", "_rev": "` + rev1 + `", "courge": "quux"}`)).
			Expect().Status(409)

		// Delete a document
		obj = e.PUT("/{db}/doc1").WithPath("db", "cozy4%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "doc1", "_rev": "` + rev2 + `", "_deleted": true}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "doc1")
		obj.Value("rev").String().NotEmpty().HasPrefix("3-")

		// Create a deleted doc (sic)
		obj = e.PUT("/{db}/doc2").WithPath("db", "cozy4%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_deleted": true}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "doc2")
		obj.Value("rev").String().NotEmpty().HasPrefix("1-")
		e.GET("/{db}").WithPath("db", "cozy4%2Fdoctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 0)
	})
}
