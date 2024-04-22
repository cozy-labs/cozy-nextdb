package core

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

type RequestIDKey struct{}

type PgxLogger struct {
	l *slog.Logger
}

func NewPgxLogger(l *slog.Logger) *PgxLogger {
	return &PgxLogger{l: l}
}

func (l *PgxLogger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
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
