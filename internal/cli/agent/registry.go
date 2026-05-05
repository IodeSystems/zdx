package agent

import (
	"fmt"
	"sort"

	"github.com/iodesystems/zdx-go/internal/config"
)

// ProviderOpts is the union of fields any provider's constructor might need.
// The manager populates the shared fields (RC, AgentCfg, SID, IssueID, Alias,
// Model, SeedPrompt) and any provider-specific extras passed by the cobra
// layer. Constructors take what they need and ignore the rest — keeping the
// signature uniform across providers without introducing a per-provider opts
// type tower.
type ProviderOpts struct {
	RC       remoteConfig
	AgentCfg config.AgentConfig
	LLMLocal config.LLMLocal

	SID        string
	IssueID    string
	Alias      string
	Model      string // resolved post-ResolveModel; empty means provider picks
	Complexity string // canonical tier (NormalizeComplexity-d) — providers may use this for full endpoint resolution
	SeedPrompt string

	// claude-specific (ignored by other providers)
	Chrome  bool
	Srcless bool

	// opencode/local-specific (ignored by claude)
	MaxTurns int
}

// ProviderConstructor builds an AgentProvider ready to be passed to
// RunLifecycle. Constructors must be cheap (no I/O); any I/O happens inside
// the adapter's Start().
type ProviderConstructor func(opts ProviderOpts) (AgentProvider, error)

var providerRegistry = map[string]ProviderConstructor{}

// RegisterProvider wires a provider name (e.g. "claude") to a constructor.
// Called from each provider file's init() so the registry is populated before
// AgentCmd is built. Re-registering the same name overwrites — useful in
// tests, harmless in production where each provider registers exactly once.
func RegisterProvider(name string, ctor ProviderConstructor) {
	providerRegistry[name] = ctor
}

// LookupProvider returns the constructor for name, or an error listing the
// known providers when name is empty / unknown.
func LookupProvider(name string) (ProviderConstructor, error) {
	if name == "" {
		return nil, fmt.Errorf("--provider is required (one of: %s)", providerNames())
	}
	ctor, ok := providerRegistry[name]
	if !ok {
		return nil, fmt.Errorf("unknown --provider %q (one of: %s)", name, providerNames())
	}
	return ctor, nil
}

// providerNames returns the registered provider names, sorted for stable
// error messages.
func providerNames() string {
	names := make([]string, 0, len(providerRegistry))
	for n := range providerRegistry {
		names = append(names, n)
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += "|"
		}
		out += n
	}
	return out
}
