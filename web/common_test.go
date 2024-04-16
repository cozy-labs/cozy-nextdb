package web

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gavv/httpexpect/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func preparePG(t *testing.T) *postgres.PostgresContainer {
	t.Helper()

	ctx := context.Background()
	image := testcontainers.WithImage("docker.io/postgres:16-alpine")
	ready := testcontainers.WithWaitStrategy(
		wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(5 * time.Second))
	container, err := postgres.RunContainer(ctx, image, ready)
	require.NoError(t, err, "Cannot run the postgres container")

	// TODO Run migrations on the database
	err = container.Snapshot(ctx, postgres.WithSnapshotName("test-snapshot"))
	require.NoError(t, err, "Cannot create a postgres snapshot")
	t.Cleanup(func() {
		require.NoError(t, container.Terminate(ctx), "failed to terminate container")
	})

	return container
}

func connectToPG(t *testing.T, container *postgres.PostgresContainer) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	pgURL, err := container.ConnectionString(ctx)
	require.NoError(t, err, "Cannot get connection string for PostgreSQL container")
	pg, err := pgxpool.New(ctx, pgURL)
	require.NoError(t, err, "Cannot create a pgxpool")
	t.Cleanup(func() {
		pg.Close()
		require.NoError(t, container.Restore(ctx), "Cannot restore the container")
	})

	return pg
}

func launchTestServer(t *testing.T, pg *pgxpool.Pool) *httpexpect.Expect {
	t.Helper()

	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.TimeOnly,
	}))
	handler := Handler(&Server{Logger: logger, PG: pg})
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})

	return httpexpect.Default(t, ts.URL).Builder(func(req *httpexpect.Request) {
		req.WithTransformer(func(r *http.Request) {
			// https://github.com/golang/go/commit/874a605af0764a8f340c3de65406963f514e21bc
			r.URL.RawPath = r.URL.Path
			r.URL.Path = strings.ReplaceAll(r.URL.Path, "%2F", "/")
		})
	})
}

func TestCommon(t *testing.T) {
	t.Parallel()
	container := preparePG(t)

	t.Run("Test the /status endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
		e.GET("/status").Expect().Status(200).
			JSON().Object().HasValue("status", "OK")
	})

	t.Run("Test the PUT /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
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
		e.PUT("/{db}").WithPath("db", "prefix%2Fdoctype1").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.PUT("/{db}").WithPath("db", "prefix%2Fdoctype2").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
	})

	t.Run("Test the GET/HEAD /:db endpoint", func(t *testing.T) {
		e := launchTestServer(t, connectToPG(t, container))
		e.PUT("/{db}").WithPath("db", "cozydb%2Fdoctype").
			Expect().Status(201).
			JSON().Object().HasValue("ok", true)
		e.HEAD("/{db}").WithPath("db", "cozydb%2Fdoctype").
			Expect().Status(200)
		e.GET("/{db}").WithPath("db", "cozydb%2Fdoctype").
			Expect().Status(200).
			JSON().Object().HasValue("doc_count", 0)

		e.GET("/{db}").WithPath("db", "cozydb%2Fno_such_doctype").
			Expect().Status(404)
		e.GET("/{db}").WithPath("db", "no_such_prefix%2Fdoctype").
			Expect().Status(404)
	})
}
