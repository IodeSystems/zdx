package handlers

import (
	"context"
	"fmt"
	"math"
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
func reviewIDStr(n int32) string    { return strconv.FormatInt(int64(n), 10) }

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

func fmtTSAny(v interface{}) string {
	switch t := v.(type) {
	case pgtype.Timestamptz:
		return fmtTS(t)
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	default:
		return ""
	}
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

// projectBySlugGetter is the minimal db surface getProject needs; *db.Queries
// implements it. Extracted so handler tests can stub the lookup.
type projectBySlugGetter interface {
	GetProjectBySlug(ctx context.Context, slug string) (db.ZdxProject, error)
}

func getProject(ctx context.Context, q projectBySlugGetter, slug string) (db.ZdxProject, error) {
	if !IsInProjectScope(ctx, slug) {
		return db.ZdxProject{}, huma.NewError(http.StatusForbidden, "project not in token scope")
	}
	p, err := q.GetProjectBySlug(ctx, slug)
	if err != nil {
		return p, huma.NewError(http.StatusNotFound, "project not found: "+slug)
	}
	return p, nil
}

// pctRound2 returns 100*num/denom rounded to 2 decimals; 0 when denom == 0.
func pctRound2(num, denom int64) float64 {
	if denom == 0 {
		return 0
	}
	return round2(float64(num) / float64(denom) * 100)
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// buildOwnerSnapshotMetrics assembles the IS-589 metric map from query rows.
// Pulled out of the handler so it can be unit-tested without DB plumbing.
func buildOwnerSnapshotMetrics(
	feats db.OwnerSnapshotFeaturesRow,
	cov db.OwnerSnapshotSpecCoverageRow,
	velo db.OwnerSnapshotPeriodRow,
	mat db.OwnerSnapshotMaturityRow,
	avgResolutionMinutes float64,
) map[string]float64 {
	return map[string]float64{
		"features_total":                          float64(feats.FeaturesTotal),
		"features_with_specs":                     float64(feats.FeaturesWithSpecs),
		"features_with_demos":                     float64(feats.FeaturesWithDemos),
		"spec_coverage_must_pct":                  pctRound2(cov.SpecsMustImplemented, cov.SpecsMustTotal),
		"spec_coverage_should_pct":                pctRound2(cov.SpecsShouldImplemented, cov.SpecsShouldTotal),
		"spec_coverage_nice_pct":                  pctRound2(cov.SpecsNiceImplemented, cov.SpecsNiceTotal),
		"specs_implemented_per_period":            float64(velo.SpecsImplementedInPeriod),
		"issues_closed_per_period":                float64(velo.IssuesClosedInPeriod),
		"maturity_rungs_passed":                   float64(mat.RungsPassed),
		"maturity_rungs_failed":                   float64(mat.RungsFailed),
		"blocker_question_avg_resolution_minutes": round2(avgResolutionMinutes),
	}
}
