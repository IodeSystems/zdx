// Package escalation codifies the standard tech→user and product→user
// escalation triggers used by the IS-1090 tier-routed routing system.
//
// IS-1092's escalation-graph validator and IS-1094's tech-tier rubric
// consume these lists to gate free-text --reason on those routes: the
// reason must contain (case-insensitive substring) at least one entry
// from the relevant list.
//
// TechToUserStandard is extensible per-project via the
// escalations.user_only []string field in .zdx/config.yaml; see
// MergedTechToUser. ProductToUserStandard is not project-extensible:
// product-level escalation triggers are fixed by the SDLC model.
package escalation

import (
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
)

// TechToUserStandard lists the codified high-risk technical changes that
// warrant a tech→user escalation. Substring match is case-insensitive;
// callers are expected to compare against the free-text --reason field.
var TechToUserStandard = []string{
	"db engine swap",
	"server programming-language switch",
	"multi-tenant introduction",
	"multi-tenant removal",
	"api paradigm shift",          // REST↔GraphQL, sync↔streaming
	"key external service change", // auth provider, message bus
	"irreversible data migration", // one-way, cross-format
}

// ProductToUserStandard lists the codified vision/goal-level conflicts
// that warrant a product→user escalation. Not project-extensible.
var ProductToUserStandard = []string{
	"two-goal unresolvable conflict",
	"feature without candidate goal",
	"explicit strategic redirection",
}

// MergedTechToUser returns the union of TechToUserStandard with any
// per-project additions declared under escalations.user_only in
// .zdx/config.yaml. Entries are trimmed and case-insensitive-deduped;
// the standard entries always come first so the project additions are
// strictly an extension, not an override.
func MergedTechToUser(cfg *config.Config) []string {
	seen := make(map[string]struct{}, len(TechToUserStandard))
	out := make([]string, 0, len(TechToUserStandard))
	for _, e := range TechToUserStandard {
		key := strings.ToLower(strings.TrimSpace(e))
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, e)
	}
	if cfg == nil {
		return out
	}
	for _, e := range cfg.Escalations.UserOnly {
		trimmed := strings.TrimSpace(e)
		key := strings.ToLower(trimmed)
		if key == "" {
			continue
		}
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}

// MatchesTechToUser reports whether reason contains (case-insensitive
// substring) any entry returned by MergedTechToUser(cfg). Pass cfg=nil
// to match against the standard list only. Empty reason never matches.
func MatchesTechToUser(reason string, cfg *config.Config) bool {
	return matchesAny(reason, MergedTechToUser(cfg))
}

// MatchesProductToUser reports whether reason contains (case-insensitive
// substring) any entry in ProductToUserStandard. Empty reason never matches.
func MatchesProductToUser(reason string) bool {
	return matchesAny(reason, ProductToUserStandard)
}

func matchesAny(reason string, entries []string) bool {
	r := strings.ToLower(strings.TrimSpace(reason))
	if r == "" {
		return false
	}
	for _, e := range entries {
		k := strings.ToLower(strings.TrimSpace(e))
		if k == "" {
			continue
		}
		if strings.Contains(r, k) {
			return true
		}
	}
	return false
}
