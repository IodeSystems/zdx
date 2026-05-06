// Package streams provides the StreamHandler registry that the IS-619
// generic poller (T2) dispatches to by target_type. Each target_type
// (proposal, task, journal, review, question, ...) registers a handler
// that knows how to fetch the canonical body the comment classifier
// evaluates against and how to apply the verdict=addressed side-effect.
//
// Handlers self-register via package init in their owning package
// (e.g. internal/events/streams/proposal). The poller side-effect
// imports those packages to populate the registry.
package streams

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// ErrUnknownTargetType is returned by Lookup when no handler is
// registered for the requested target_type.
var ErrUnknownTargetType = errors.New("streams: unknown target_type")

// StreamHandler processes user-authored events on a target_type-specific
// stream. Implementations register at package init time; the generic
// poller in IS-619-T2 dispatches to them by TargetType.
type StreamHandler interface {
	TargetType() string

	// FetchBody returns the canonical text the comments are evaluated
	// against (e.g. proposal body, task description). The poller passes
	// it to the LLM classifier verbatim.
	FetchBody(ctx context.Context, c *cli.Client, slug, targetID string) (string, error)

	// ApplyAddressed runs the side-effect for verdict=addressed (e.g.
	// proposal posts a revision event; task may update status). Called
	// before the verdict is posted so the revision exists in the stream
	// ordering. Returning err aborts that comment, others continue.
	ApplyAddressed(ctx context.Context, c *cli.Client, slug, targetID string, addressingEventID int64, revisedBody string) error
}

var registry = map[string]StreamHandler{}

// Register adds h to the package-level registry. Panics if a handler
// for h.TargetType() is already registered — duplicate registration is
// a programming mistake worth surfacing at startup.
func Register(h StreamHandler) {
	t := h.TargetType()
	if t == "" {
		panic("streams: Register called with empty TargetType")
	}
	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("streams: duplicate registration for target_type %q", t))
	}
	registry[t] = h
}

// Lookup returns the handler for targetType, or ErrUnknownTargetType if
// none is registered.
func Lookup(targetType string) (StreamHandler, error) {
	h, ok := registry[targetType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownTargetType, targetType)
	}
	return h, nil
}

// All returns every registered handler, sorted by target_type for
// deterministic diagnostics output.
func All() []StreamHandler {
	out := make([]StreamHandler, 0, len(registry))
	for _, h := range registry {
		out = append(out, h)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TargetType() < out[j].TargetType() })
	return out
}
