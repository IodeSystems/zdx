package agent

import (
	"fmt"
	"sort"

	"github.com/iodesystems/zdx-go/internal/cli/agent/tracelog"
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
	Chrome        bool
	Srcless       bool
	WorkDir       string // global agent work directory (srcless mode)
	KeepContainer bool   // skip docker --rm for debugging

	// opencode/local-specific (ignored by claude)
	MaxTurns int

	// MCPCommand, when non-empty, makes opencode/local dispatch tool
	// calls through a remote MCP server spawned via this argv (typically
	// `docker exec -i <container> dx-agent --mcp-stdio`) instead of the
	// in-process fs+shell tools. Filesystem and shell isolation come from
	// wherever the subprocess runs — host, container, or anywhere
	// CommandTransport can reach over stdio.
	MCPCommand []string

	// ClusterID, when non-empty, is stamped as a tag on this loop's tracelog
	// stream so orchestrator and per-slot loops share a correlation key. Set
	// by runMCPContainerLoop to opts.Alias (the orchestrator alias) before
	// launching slot loops; the slots have their own alias=<base>-<N> but
	// the shared cluster_id makes the whole cluster filterable as one chain.
	ClusterID string

	// Concurrency controls how many slot containers `dx agent loop
	// --container=docker` spins up within this single process. Default 1
	// (sequential, single agent — what most operators want). Setting >1 is
	// an explicit opt-in to in-process fan-out; for normal parallel work,
	// run multiple `dx agent loop` processes instead so each agent is its
	// own visible OS process / alias / claim stream. NOT read from
	// agent.max_worktrees in config — that field is the project-wide
	// concurrent-agents ceiling consulted by `dx agent start`, not a slot
	// count for the loop.
	Concurrency int

	// TraceID is the per-session trace correlation ID. Populated by
	// RunManagedSession from the session tracelog's TraceID() right after
	// tracelog setup, before provider construction. Propagates two ways:
	//   1. ZDX_TRACE_ID env var on the spawned claude/opencode subprocess
	//      (via buildClaudeEnv) → its outbound dx CLI calls stamp
	//      X-ZDX-Trace-Id header → server's apiKeyMiddleware reads it →
	//      ctxTraceIDVal serves it to traceEvent → mutations land in
	//      zdx_log_events with trace_id matching the agent's session.
	//   2. `docker exec -e ZDX_TRACE_ID=...` injected into the in-slot
	//      MCP dispatch argv so in-slot dx CLI calls inherit the same
	//      trace_id (Bash tool runs `dx ...` inside the slot; without
	//      this, host-side trace_id stops at the slot boundary).
	//
	// Single `dx log tail --tag trace_id=<X>` filter then surfaces the
	// agent's whole flow: loop scaffolding events + claude turns +
	// server-side mutations the agent caused.
	TraceID string

	// Global=true registers the agent in the server-wide pool (no project
	// binding) instead of under opts.RC.slug. Surfaced in /api/agents and
	// the top-level /agents UI; assignment to a specific project happens
	// via separate API later (phase 2). Implies the agent has no project
	// queue to claim from until assigned.
	Global bool

	// Idle=true registers the agent and holds the WS connection but
	// doesn't start the work loop. Operator activates via UI or
	// (eventually) a server-pushed control command. Useful for global
	// agents that are waiting for assignment, and for project-scoped
	// agents an operator wants to "have ready" before dispatching work.
	Idle bool

	// TraceLog is the session-scoped structured logger (initialized in
	// DispatchSingle / managed wrappers). When non-nil, providers and the
	// shared lifecycle code emit setup events (worktree.create,
	// container.start, mcp.attach, setup.start, setup.done) through it so
	// `.zdx/logs/agent.log` and the server log stream surface the
	// pre-first-turn phase. Nil-safe: callers must check before use.
	TraceLog *tracelog.Logger
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
