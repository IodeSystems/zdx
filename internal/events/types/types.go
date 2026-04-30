// Package types holds renderer implementations for the unified event stream.
//
// Each renderer self-registers with internal/events via package init. To
// populate the registry, side-effect import this package:
//
//	import _ "github.com/iodesystems/zdx-go/internal/events/types"
//
// Server (IS-613) and tests use that import to ensure all known event_types
// resolve via events.Lookup.
package types
