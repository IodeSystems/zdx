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
