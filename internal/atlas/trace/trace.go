package trace

import (
	"context"
	"fmt"
	"runtime"
)

const maxEntries = 50

// Entry is a single debug trace capture.
type Entry struct {
	Coderef string `json:"coderef"`
	Label   string `json:"label"`
	Value   any    `json:"value"`
}

type accumulator struct {
	entries []Entry
}

type ctxKey struct{}

// Start attaches a new accumulator to ctx. Only call when the caller has
// verified that debug output is requested; otherwise Note is a no-op.
func Start(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxKey{}, &accumulator{})
}

// Note records a trace entry at the call site. Short-circuits when no
// accumulator is present (Start was not called) or the cap is reached.
func Note(ctx context.Context, label string, value any) {
	acc, _ := ctx.Value(ctxKey{}).(*accumulator)
	if acc == nil || len(acc.entries) >= maxEntries {
		return
	}
	_, file, line, _ := runtime.Caller(1)
	acc.entries = append(acc.entries, Entry{
		Coderef: fmt.Sprintf("%s:%d", file, line),
		Label:   label,
		Value:   value,
	})
}

// From returns accumulated entries, nil if none.
func From(ctx context.Context) []Entry {
	acc, _ := ctx.Value(ctxKey{}).(*accumulator)
	if acc == nil {
		return nil
	}
	return acc.entries
}
