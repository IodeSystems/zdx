package agent

import (
	"embed"
	"fmt"
	"sort"
	"strings"
)

// Persona is the role-and-permission tier an agent session runs as. The
// matrix and escalation graph live in IS-1090; this package owns the flag
// validation and the per-persona system-prompt block that gets prepended
// to the provider's existing prompt.
const (
	PersonaDev      = "dev"
	PersonaTech     = "tech"
	PersonaReviewer = "reviewer"
	PersonaProduct  = "product"

	// DefaultPersona is applied when --persona is omitted. Per IS-1090 Q4
	// (awaiting product confirmation) we proceed with dev — it matches
	// historical claim distribution.
	DefaultPersona = PersonaDev
)

//go:embed prompts/*.md
var personaPrompts embed.FS

// allowedPersonas is the canonical set in matrix order. Lookup is O(n) but
// n=4 forever, so a slice is simpler than a map and keeps error messages
// deterministic.
var allowedPersonas = []string{
	PersonaDev,
	PersonaTech,
	PersonaReviewer,
	PersonaProduct,
}

// NormalizePersona validates and canonicalizes a --persona value. Empty
// returns DefaultPersona. Anything outside the four allowed values errors —
// no silent fallback (IS-1096).
func NormalizePersona(s string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(s))
	if v == "" {
		return DefaultPersona, nil
	}
	for _, p := range allowedPersonas {
		if v == p {
			return p, nil
		}
	}
	sorted := make([]string, len(allowedPersonas))
	copy(sorted, allowedPersonas)
	sort.Strings(sorted)
	return "", fmt.Errorf("--persona must be one of %s (got %q)", strings.Join(sorted, "|"), s)
}

// PersonaPrompt returns the per-persona prompt block (the contents of
// internal/cli/agent/prompts/<persona>.md). The caller prepends this to
// whatever provider-specific system prompt is already in use.
//
// An unknown persona is treated as a programming error — callers should
// have run NormalizePersona first; we panic-equivalent by returning an
// error so the agent surfaces it instead of silently picking dev.
func PersonaPrompt(persona string) (string, error) {
	p, err := NormalizePersona(persona)
	if err != nil {
		return "", err
	}
	b, err := personaPrompts.ReadFile("prompts/" + p + ".md")
	if err != nil {
		return "", fmt.Errorf("load persona prompt %s: %w", p, err)
	}
	return string(b), nil
}
