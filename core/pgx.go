package core

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
)

type RequestIDKey struct{}

func NewPgxConfig(pgURL string, logger *slog.Logger) (*pgxpool.Config, error) {
	config, err := pgxpool.ParseConfig(pgURL)
	if err != nil {
		return nil, err
	}
	// Trace the SQL queries and send the result in logs.
	config.ConnConfig.Tracer = &tracelog.TraceLog{
		Logger:   &pgxLogger{l: logger},
		LogLevel: tracelog.LogLevelInfo,
	}
	// Disable prepared statements. Prepared statements are bound to a table
	// and a connection. With many tables and a pool of connections, they take
	// a significant amount of memory but are seldom used. So, it looks better
	// to disable them.
	config.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	// A map[string]any can be jsonb, but also hstore. Pgx can ask the postgres
	// server to know the SQL types, but it takes a round-trip, that we can
	// avoid by using a default type.
	config.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		tm := conn.TypeMap()
		tm.RegisterDefaultPgType(map[string]any{}, "jsonb")
		tm.RegisterDefaultPgType(RevsStruct{}, "jsonb")
		return nil
	}
	return config, nil
}

type pgxLogger struct {
	l *slog.Logger
}

func (l *pgxLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	attrs := make([]slog.Attr, 0, len(data)+1)
	for k, v := range data {
		attrs = append(attrs, slog.Any(k, v))
	}
	attrs = append(attrs,
		slog.String("nspace", "sql"),
		slog.Any("req_id", ctx.Value(RequestIDKey{})),
	)
	lvl := slog.LevelDebug
	if level == tracelog.LogLevelError {
		lvl = slog.LevelWarn
	}
	l.l.LogAttrs(ctx, lvl, msg, attrs...)
}
