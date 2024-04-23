package web

import "testing"

func TestAdvanced(t *testing.T) {
	t.Parallel()
	container := preparePG(t)
	logger := setupLogger(t)

	t.Run("Mango", func(t *testing.T) {
		e := launchTestServer(t, logger, connectToPG(t, container, logger))
		e.PUT("/{db}").WithPath("db", "mango%2Fdoctype").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.POST("/{db}").WithPath("db", "mango%2Fdoctype").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "foo", "value": "foo"}`)).
			Expect().Status(201)
		e.POST("/{db}").WithPath("db", "mango%2Fdoctype").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "bar", "value": "bar"}`)).
			Expect().Status(201)
		e.POST("/{db}").WithPath("db", "mango%2Fdoctype").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "baz", "value": "baz"}`)).
			Expect().Status(201)
		e.POST("/{db}").WithPath("db", "mango%2Fdoctype").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "qux", "value": "qux"}`)).
			Expect().Status(201)
		e.POST("/{db}").WithPath("db", "mango%2Fdoctype").
			WithHeader("Content-Type", "application/json").
			WithBytes([]byte(`{"_id": "quux", "value": "quux"}`)).
			Expect().Status(201)

		t.Run("Basic", func(t *testing.T) {
			t.Parallel()
			e := launchTestServer(t, logger, connectToPG(t, container, logger))

			// Check errors
			e.POST("/{db}/_find").WithPath("db", "mango%2Fdoctype").
				WithBytes([]byte(`not_json`)).
				Expect().Status(400)
			e.POST("/{db}/_find").WithPath("db", "no_such_cozy%2Fdoctype").
				WithHeader("Content-Type", "application/json").
				WithBytes([]byte(`{"selector": {}}`)).
				Expect().Status(404)

			obj := e.POST("/{db}/_find").WithPath("db", "mango%2Fdoctype").
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
	})
}
