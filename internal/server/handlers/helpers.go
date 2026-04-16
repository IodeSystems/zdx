package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// ── ID conversion helpers ─────────────────────────────────────────────────

func intFromPrefixed(s, prefix string) int32 {
	if !strings.HasPrefix(s, prefix) {
		return 0
	}
	n, _ := strconv.ParseInt(s[len(prefix):], 10, 32)
	return int32(n)
}

func issueIntID(id string) int32 { return intFromPrefixed(id, "IS-") }
func taskIntID(id string) int32  { return intFromPrefixed(id, "TK-") }

func issueIDFromInt(n int32) string { return fmt.Sprintf("IS-%d", n) }
func taskIDFromInt(n int32) string  { return fmt.Sprintf("TK-%d", n) }

// IssueIDFromInt is the exported form for callers outside this package.
func IssueIDFromInt(n int32) string { return issueIDFromInt(n) }

// TaskIDFromInt is the exported form for callers outside this package.
func TaskIDFromInt(n int32) string { return taskIDFromInt(n) }

func fmtTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339)
}

// ptrStr dereferences a *string, returning "" for nil.
func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// ptrInt32 dereferences a *int32, returning 0 for nil.
func ptrInt32(n *int32) int32 {
	if n == nil {
		return 0
	}
	return *n
}

func parsePage(limit, offset int32) (int32, int32) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}

func apiErr(status int, msg string) huma.StatusError {
	return huma.NewError(status, msg)
}

// APIErr is the exported alias so server-package code can produce the same
// huma error shape used by handlers.
func APIErr(status int, msg string) huma.StatusError { return apiErr(status, msg) }

func getProject(ctx context.Context, q *db.Queries, slug string) (db.ZdxProject, error) {
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		return p, huma.NewError(http.StatusNotFound, "project not found: "+slug)
	}
	return p, nil
}
