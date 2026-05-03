package doctor

import (
	"strings"
	"time"
)

// AdminScopedToken describes an API key whose project_scope IS NULL — i.e. an
// admin-unrestricted key that can act across every project. Populated from
// ListAdminScopedApiKeys.
type AdminScopedToken struct {
	ID         int32
	Name       string
	LastUsedAt time.Time // zero when the key has never been used
}

// recentAdminTokenWindow is the look-back window for the admin_tokens_quiet
// check. Admin-scoped (non-agent) keys used within this window surface as
// ActionInfo so operators can see broadly-scoped tokens that are still active.
const recentAdminTokenWindow = 7 * 24 * time.Hour

// PartitionAdminScopedTokens splits admin-scoped (project_scope IS NULL) keys
// into two doctor-relevant buckets:
//
//   - legacyAgentTokens: keys whose name begins with "agent-" — pre-IS-838
//     agent tokens that should be rotated to project-scoped tokens minted via
//     `dx token mint --project=<slug>`.
//   - recentAdminTokens: non-agent admin keys used within the trailing 7 days
//     — broadly-scoped tokens that are still hot and worth surfacing.
//
// Older/unused non-agent admin keys are intentionally silent: they exist but
// aren't actively risky enough to nag about.
func PartitionAdminScopedTokens(tokens []AdminScopedToken, now time.Time) (legacyAgentTokens, recentAdminTokens []AdminScopedToken) {
	for _, tok := range tokens {
		if strings.HasPrefix(tok.Name, "agent-") {
			legacyAgentTokens = append(legacyAgentTokens, tok)
			continue
		}
		if !tok.LastUsedAt.IsZero() && now.Sub(tok.LastUsedAt) <= recentAdminTokenWindow {
			recentAdminTokens = append(recentAdminTokens, tok)
		}
	}
	return
}
