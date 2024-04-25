package web

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime/trace"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cozy-labs/cozy-nextdb/core"
	"github.com/gavv/httpexpect/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lmittmann/tint"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var logger *slog.Logger
var pg *pgxpool.Pool

func TestMain(m *testing.M) {
	// XXX defer are not executed when os.Exit() is called, so we wrap the code
	// in another function to be able to use defer.
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()
	ctx, task := trace.NewTask(ctx, "TestMain")
	defer task.End()

	logger = setupLogger()
	container, err := preparePG(ctx)
	if err != nil {
		return -1
	}
	defer func() {
		_ = container.Terminate(ctx)
	}()

	pg, err = connectToPG(ctx, container)
	if err != nil {
		return -1
	}
	defer pg.Close()

	return m.Run()
}

func setupLogger() *slog.Logger {
	opts := &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.TimeOnly,
	}
	if os.Getenv("TEST_NEXTDB_LOG_LEVEL") == "debug" {
		opts.Level = slog.LevelDebug
	}
	return slog.New(tint.NewHandler(os.Stderr, opts))
}

func preparePG(ctx context.Context) (*postgres.PostgresContainer, error) {
	region := trace.StartRegion(ctx, "preparePG")
	defer region.End()

	pg_image := os.Getenv("TEST_NEXTDB_PG_IMAGE")
	if pg_image == "" {
		pg_image = "docker.io/postgres:16-alpine"
	}
	image := testcontainers.WithImage(pg_image)
	var tmpfs testcontainers.CustomizeRequestOption = func(req *testcontainers.GenericContainerRequest) {
		req.Tmpfs = map[string]string{"/var/lib/postgresql/data": "rw"}
	}
	ready := testcontainers.WithWaitStrategy(
		wait.ForLog("database system is ready to accept connections").
			WithOccurrence(2).
			WithStartupTimeout(5 * time.Second))
	container, err := postgres.RunContainer(ctx, image, tmpfs, ready)
	if err != nil {
		return nil, fmt.Errorf("cannot run the postgresql container: %w", err)
	}
	return container, nil
}

func connectToPG(ctx context.Context, container *postgres.PostgresContainer) (*pgxpool.Pool, error) {
	pgURL, err := container.ConnectionString(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot get connection string for PostgreSQL container: %w", err)
	}
	config, err := core.NewPgxConfig(pgURL, logger)
	if err != nil {
		return nil, fmt.Errorf("cannot parse config for pgxpool: %w", err)
	}
	pg, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("cannot parse config for pgxpool: %w", err)
	}
	return pg, nil
}

func launchTestServer(t *testing.T, ctx context.Context) *httpexpect.Expect {
	t.Helper()

	handler := Handler(&Server{Logger: logger, PG: pg})
	ts := httptest.NewUnstartedServer(handler)
	ts.Config.BaseContext = func(net.Listener) context.Context {
		return ctx
	}
	ts.Start()
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

var prefixCounter int32

func getPrefix(s string) string {
	n := atomic.AddInt32(&prefixCounter, 1)
	return fmt.Sprintf("%s%d", s, n)
}

func getDatabase(prefix, doctype string) string {
	escaped := strings.ReplaceAll(doctype, ".", "-")
	return prefix + "%2F" + escaped
}

func TestCommon(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ctx, task := trace.NewTask(ctx, "TestCommon")
	defer task.End()

	t.Run("Test the /status endpoint", func(t *testing.T) {
		t.Parallel()
		e := launchTestServer(t, ctx)
		e.GET("/status").Expect().Status(200).
			JSON().Object().HasValue("status", "OK")
	})
}
