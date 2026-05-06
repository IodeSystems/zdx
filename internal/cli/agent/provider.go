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

// Container modes exposed via --container. The flag is an *execution
// environment*, not a yes/no — "docker" runs in MCP-slot containers
// (worktree-isolated, sandboxed); "local" runs claude directly on the
// operator's host. We default to docker because container is the
// production-style execution path and operators should opt down to local
// only when debugging or when docker is unavailable.
const (
	ContainerDocker = "docker"
	ContainerLocal  = "local"

	DefaultContainer = ContainerDocker
)

// NormalizeContainer accepts the --container flag and returns the
// canonical mode. Empty / "auto" return DefaultContainer (docker today).
// "host" is a friendly synonym for "local".
func NormalizeContainer(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto", ContainerDocker:
		return ContainerDocker, nil
	case ContainerLocal, "host":
		return ContainerLocal, nil
	}
	return "", fmt.Errorf("--container must be one of docker|local (got %q)", s)
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
// errors on unknown input. Empty input or "auto" returns DefaultComplexity —
// the resolver layer can replace DefaultComplexity with config-derived values
// later without changing this signature.
//
// Accepted spellings:
//
//	auto   | (empty) — DefaultComplexity (currently "high")
//	low    | l
//	medium | med | m
//	high   | h
func NormalizeComplexity(s string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "auto":
		return DefaultComplexity, nil
	case "low", "l":
		return ComplexityLow, nil
	case "medium", "med", "m":
		return ComplexityMedium, nil
	case "high", "h":
		return ComplexityHigh, nil
	}
	return "", fmt.Errorf("--complexity must be one of auto|low|medium|high (got %q)", s)
}
