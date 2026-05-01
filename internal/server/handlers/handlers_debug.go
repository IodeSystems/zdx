package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/atlas/trace"
)

// DebugOutput wraps the trace entries returned when ?debug=1 is present.
type DebugOutput struct {
	Trace []trace.Entry `json:"trace"`
}

// debugStart activates the trace accumulator when the debug flag is present.
// Returns (updatedCtx, true, nil) on success, (ctx, false, nil) when no debug
// flag, or (ctx, false, 403) when debug is requested without admin role.
func debugStart(ctx context.Context, debug, xAtlasDebug string) (context.Context, bool, error) {
	if debug != "1" && xAtlasDebug != "1" {
		return ctx, false, nil
	}
	if ctxUserRoleVal(ctx) != "admin" {
		return ctx, false, huma.NewError(http.StatusForbidden, "debug requires admin role")
	}
	return trace.Start(ctx), true, nil
}

// debugOutput returns a populated DebugOutput when trace entries exist, nil otherwise.
func debugOutput(ctx context.Context) *DebugOutput {
	entries := trace.From(ctx)
	if len(entries) == 0 {
		return nil
	}
	return &DebugOutput{Trace: entries}
}
