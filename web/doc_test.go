package web

import (
	"strings"
	"testing"
)

func TestDoc(t *testing.T) {
	t.Parallel()
	container := preparePG(t)

	t.Run("Test the POST /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
		e.PUT("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)

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

		obj := e.POST("/{db}").WithPath("db", "cozy1%2Fdoctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.Value("id").String().NotEmpty().Length().IsEqual(32)
		obj.Value("rev").String().NotEmpty().HasPrefix("1-").
			Length().IsEqual(34) // 2 bytes for 1- and 32 bytes for the checksum

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
	})

	t.Run("Test the GET /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
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

		e.HEAD("/{db}/"+id).WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(200).
			Header("ETag").IsEqual(rev)
		obj = e.GET("/{db}/"+id).WithPath("db", "cozy2%2Fdoctype1").
			Expect().Status(200).
			JSON().Object()
		obj.HasValue("_id", id)
		obj.HasValue("_rev", rev)
		obj.HasValue("foo", "bar")

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
}
