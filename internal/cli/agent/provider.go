package agent

import (
	"context"
	"fmt"
	"strings"
)

// AgentProvider extends AgentAdapter with model resolution. Each provider
// (claude, opencode, local) owns the mapping from a complexity tier to the
// concrete model identifier it will pass to its underlying runtime.
//
// The shape of "the agent CLI" is uniform — the operator picks a tier; the
// provider knows which model implements that tier. New providers plug in by
// implementing this interface; no manager-side changes are required.
type AgentProvider interface {
	AgentAdapter

	// ResolveModel maps a complexity tier (low|medium|high) to a concrete
	// model identifier. Empty complexity is treated as DefaultComplexity.
	// An unknown tier returns an error rather than a silent fallback.
	ResolveModel(ctx context.Context, complexity string) (string, error)
}

// LoopProvider is the optional interface a provider implements when it has
// its own loop runtime that the standard RunManagedLoop can't represent.
// Today only claude needs this — its loop creates a fresh worktree per
// session, rotates models across iterations, and recovers stalled sessions
// via transcript summary, none of which other providers do.
//
// When --provider=X is dispatched in loop mode, the manager checks if the
// provider implements LoopProvider; if so, calls RunLoop instead of
// RunManagedLoop. Plain providers (opencode, local) get the universal loop.
type LoopProvider interface {
	AgentProvider
	RunLoop(ctx context.Context, opts ProviderOpts) error
}

// ContainerProvider is the optional interface for providers that support
// docker-orchestrated parallel sessions (one container per slot, restart
// on exit). Claude's --container mode is the only consumer today.
//
// dx agent loop --container --provider=X errors when X doesn't implement
// ContainerProvider. KeepContainer in opts toggles --rm.
type ContainerProvider interface {
	AgentProvider
	RunContainerLoop(ctx context.Context, opts ProviderOpts) error
}

// Complexity tiers exposed via --complexity. See IS-1031 for the planned
// expansion to {low, medium, high, xhigh, max}; for now we settle on three.
const (
	ComplexityLow    = "low"
	ComplexityMedium = "medium"
	ComplexityHigh   = "high"

	// DefaultComplexity is the value applied when --complexity is omitted.
	// We bias toward "high" so the operator opts down for cheap tasks
	// instead of opting up for capable ones.
	DefaultComplexity = ComplexityHigh
)

// NormalizeComplexity collapses common spellings to a canonical tier and
// errors on unknown input. Empty input returns DefaultComplexity.
//
// Accepted spellings:
//
//	low    | l
//	medium | med | m
//	high   | h
func NormalizeComplexity(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "":
		return DefaultComplexity, nil
	case "low", "l":
		return ComplexityLow, nil
	case "medium", "med", "m":
		return ComplexityMedium, nil
	case "high", "h":
		return ComplexityHigh, nil
	}
	return "", fmt.Errorf("--complexity must be one of low|medium|high (got %q)", s)
}
