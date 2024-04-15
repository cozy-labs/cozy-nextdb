package web

import "testing"

func TestDoc(t *testing.T) {
	t.Parallel()
	container := preparePG(t)

	t.Run("Test the POST /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
		e.PUT("/{db}").WithPath("db", "cozy1/doctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)

		e.POST("/{db}").WithPath("db", "cozy1/doctype1").
			WithBytes([]byte(`not_json`)).
			Expect().Status(400)
		e.POST("/{db}").WithPath("db", "cozy2/doctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such cozy"}`)).
			Expect().Status(404)
		e.POST("/{db}").WithPath("db", "cozy1/doctype2").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"comment": "no such doctype"}`)).
			Expect().Status(404)
		e.POST("/{db}").WithPath("db", "cozy1/doctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "foo", "_rev": "2-345"}`)).
			Expect().Status(409)

		obj := e.POST("/{db}").WithPath("db", "cozy1/doctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"foo": "bar"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.Value("id").String().NotEmpty()
		obj.Value("rev").String().NotEmpty().HasPrefix("1-")

		obj = e.POST("/{db}").WithPath("db", "cozy1/doctype1").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "myid"}`)).
			Expect().Status(201).
			JSON().Object()
		obj.HasValue("ok", true)
		obj.HasValue("id", "myid")
		obj.Value("rev").String().NotEmpty().HasPrefix("1-")
		e.GET("/{db}").WithPath("db", "cozy1/doctype1").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 2)
	})
}
