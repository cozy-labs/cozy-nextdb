package web

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"os"
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

func openDB(t *testing.T, container *postgres.PostgresContainer) *pgxpool.Pool {
	t.Helper()

	ctx := context.Background()
	dbURL, err := container.ConnectionString(ctx)
	require.NoError(t, err, "Cannot get connection string for db")
	db, err := pgxpool.New(ctx, dbURL)
	require.NoError(t, err, "Cannot create a pgxpool")
	t.Cleanup(func() {
		db.Close()
		require.NoError(t, container.Restore(ctx), "Cannot restore the container")
	})

	return db
}

func launchTestServer(t *testing.T, db *pgxpool.Pool) *httpexpect.Expect {
	t.Helper()

	logger := slog.New(tint.NewHandler(os.Stderr, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.TimeOnly,
	}))
	handler := Handler(&Server{Logger: logger, DB: db})
	ts := httptest.NewServer(handler)
	t.Cleanup(func() {
		ts.Close()
	})

	return httpexpect.Default(t, ts.URL)
}

func TestCommon(t *testing.T) {
	t.Parallel()
	container := preparePG(t)

	t.Run("Test inserting a user", func(t *testing.T) {
		db := openDB(t, container)
		e := launchTestServer(t, db)

		e.GET("/status").Expect().Status(200).
			JSON().Object().HasValue("status", "OK")
	})
}
