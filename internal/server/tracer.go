package server

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// QueryTracer implements pgx.QueryTracer to capture per-request SQL timings.
// Wire it into the pool via pgxpool.Config.ConnConfig.Tracer before creating
// the pool. The server's sqlTimingMiddleware injects a *sqlTimingSlice into
// each request context; TraceQueryEnd deposits timings there so the middleware
// can persist them after the handler returns.
type QueryTracer struct{}

type queryStartInfo struct {
	start time.Time
	name  string
}

func (QueryTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	return context.WithValue(ctx, ctxQueryStart, queryStartInfo{
		start: time.Now(),
		name:  sqlQueryName(data.SQL),
	})
}

func (QueryTracer) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryEndData) {
	info, ok := ctx.Value(ctxQueryStart).(queryStartInfo)
	if !ok {
		return
	}
	acc, ok := ctx.Value(ctxSQLTimings).(*sqlTimingSlice)
	if !ok {
		return
	}
	ms := int32(time.Since(info.start).Milliseconds()) //nolint:gosec
	acc.items = append(acc.items, sqlTiming{name: info.name, durationMs: ms})
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
