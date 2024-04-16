package core

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

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
	attrs = append(attrs, slog.String("nspace", "sql"))
	l.l.LogAttrs(context.Background(), slog.LevelDebug, msg, attrs...)
}
