package handlers

import "context"

type contextKey int

const (
	CtxAPIKeyID   contextKey = 1
	CtxUserID     contextKey = 2
	CtxQueryStart contextKey = 3
	CtxSource     contextKey = 4
	CtxUserRole   contextKey = 5
	CtxSkipTiming contextKey = 6
	CtxAgentID    contextKey = 7
	CtxSessionID    contextKey = 8
	CtxProjectScope contextKey = 9
)

func ctxUserIDVal(ctx context.Context) int32 {
	v, _ := ctx.Value(CtxUserID).(int32)
	return v
}

func ctxUserRoleVal(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserRole).(string)
	return v
}

func ctxAgentIDVal(ctx context.Context) string {
	v, _ := ctx.Value(CtxAgentID).(string)
	return v
}

func ctxSessionIDVal(ctx context.Context) string {
	v, _ := ctx.Value(CtxSessionID).(string)
	return v
}

// UserIDFromContext is an exported accessor for server-side middleware and other
// packages that need to read the authenticated user ID from the request context.
func UserIDFromContext(ctx context.Context) int32 { return ctxUserIDVal(ctx) }

// UserRoleFromContext is the exported counterpart to ctxUserRoleVal.
func UserRoleFromContext(ctx context.Context) string { return ctxUserRoleVal(ctx) }

// SourceFromContext returns the source label stamped on ctx by sourceMiddleware
// or WithSource, falling back to "background".
func SourceFromContext(ctx context.Context) string {
	if s, ok := ctx.Value(CtxSource).(string); ok && s != "" {
		return s
	}
	return "background"
}

// WithSource labels ctx so queries running under it are attributed to source
// instead of "background". Use at the entry of cron jobs, CLI commands, or
// any long-lived goroutine you want visible in the timings dashboard.
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, CtxSource, source)
}

// WithoutTiming returns a ctx that the QueryTracer will skip entirely.
func WithoutTiming(ctx context.Context) context.Context {
	return context.WithValue(ctx, CtxSkipTiming, true)
}

// SkipTimingFromContext reports whether ctx has been marked to skip tracer timing.
func SkipTimingFromContext(ctx context.Context) bool {
	v, _ := ctx.Value(CtxSkipTiming).(bool)
	return v
}

// ProjectScopeFromContext returns the project_scope slice stored by apiKeyMiddleware.
// Returns nil if no scope is set (key is unrestricted).
func ProjectScopeFromContext(ctx context.Context) []string {
	v, _ := ctx.Value(CtxProjectScope).([]string)
	return v
}

// IsInProjectScope reports whether slug is within the key's project scope.
// An empty or nil scope means unrestricted — returns true for any slug.
func IsInProjectScope(ctx context.Context, slug string) bool {
	scope := ProjectScopeFromContext(ctx)
	if len(scope) == 0 {
		return true
	}
	for _, s := range scope {
		if s == slug {
			return true
		}
	}
	return false
}
