package server

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryTracer implements pgx.QueryTracer. Every query that crosses the pool
// is timed and pushed to Sink. Source is read from ctx (stamped by
// sourceMiddleware for HTTP, by WithSource for background work); if missing,
// the query is still recorded with source="background" so nothing disappears.
// A ctx carrying ctxSkipTiming is ignored — used by the drainer itself.
type QueryTracer struct {
	Sink timingSink
}

type queryStartInfo struct {
	start time.Time
	name  string
}

func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if v, _ := ctx.Value(ctxSkipTiming).(bool); v {
		return ctx
	}
	return context.WithValue(ctx, ctxQueryStart, queryStartInfo{
		start: time.Now(),
		name:  sqlQueryName(data.SQL),
	})
}

func (t QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	info, ok := ctx.Value(ctxQueryStart).(queryStartInfo)
	if !ok {
		return
	}
	ms := int32(time.Since(info.start).Milliseconds()) //nolint:gosec
	if ms < 1 {
		return
	}
	t.Sink.send(Timing{
		Name:       info.name,
		Source:     sourceFromCtx(ctx),
		DurationMs: ms,
	})
}

// sqlQueryName extracts a short stable name from a SQL string.
// sqlc prepends "-- name: QueryName :kind\n" to every query; we use that.
// Falls back to the first 60 chars of SQL for unrecognized formats.
func sqlQueryName(sql string) string {
	if idx := strings.IndexByte(sql, '\n'); idx > 0 {
		first := sql[:idx]
		if strings.HasPrefix(first, "-- name: ") {
			parts := strings.Fields(first)
			if len(parts) >= 3 {
				return "sql:" + parts[2]
			}
		}
	}
	if len(sql) > 60 {
		return "sql:" + strings.TrimSpace(sql[:60])
	}
	return "sql:" + strings.TrimSpace(sql)
}
