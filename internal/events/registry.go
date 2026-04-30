// Package events provides the Renderer registry for the unified event stream
// (IS-610 vertical 2). Each event_type stored on zdx_events has a Renderer
// that knows how to validate the JSON detail blob, summarize it for agent
// context, expose a structured Detail for inspect/API, and name the React
// component that renders it on the UI.
//
// Renderers self-register via package init in internal/events/types.
// Server handlers (IS-613) and tests side-effect import that package to
// populate the registry.
package events

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
)

// Audience constants identify which surfaces a renderer's events are visible on.
const (
	AudienceUI    = "ui"
	AudienceAgent = "agent"
)

// ErrUnknownEventType is returned by Lookup when no renderer is registered
// for the requested event_type.
var ErrUnknownEventType = errors.New("events: unknown event_type")

// Renderer describes a single event_type. Implementations are stateless and
// are registered once at package init time.
type Renderer interface {
	EventType() string
	Audience() []string
	Validate(detail json.RawMessage) error
	Summary(detail json.RawMessage) (string, error)
	Detail(detail json.RawMessage) (any, error)
	ReactComponentName() string
}

var registry = map[string]Renderer{}

// Register adds r to the package-level registry. Panics if a renderer for
// r.EventType() is already registered — duplicate registration is a
// programming mistake worth surfacing at startup.
func Register(r Renderer) {
	t := r.EventType()
	if t == "" {
		panic("events: Register called with empty EventType")
	}
	if _, exists := registry[t]; exists {
		panic(fmt.Sprintf("events: duplicate registration for event_type %q", t))
	}
	registry[t] = r
}

// Lookup returns the renderer for eventType, or ErrUnknownEventType if none
// is registered.
func Lookup(eventType string) (Renderer, error) {
	r, ok := registry[eventType]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownEventType, eventType)
	}
	return r, nil
}

// All returns every registered renderer, sorted by event_type for
// deterministic diagnostics output.
func All() []Renderer {
	out := make([]Renderer, 0, len(registry))
	for _, r := range registry {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EventType() < out[j].EventType() })
	return out
}
