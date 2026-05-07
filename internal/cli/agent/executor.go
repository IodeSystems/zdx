package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iodesystems/zdx-go/internal/config"
)

func nowRFC3339() string { return time.Now().Format(time.RFC3339) }

// Workspace is a provisioned execution context for one session or one
// claim/work loop. The dispatch layer hands this to drivers
// (DispatchSingle / DispatchLoop) without caring how it was provisioned —
// host cwd vs slot container is the executor's concern.
//
// MCPCommand is the argv for spawning the in-workspace MCP server.
// Empty means the session uses the host's default MCP wiring (no
// per-session MCP override). Set for ContainerExecutor's slot model
// (`docker exec -i <slot> /workspace/bin/dx-agent --mcp-stdio`).
//
// Cleanup is idempotent and called via defer; safe to invoke after a
// failed session.
type Workspace struct {
	Name       string
	MCPCommand []string
	Cleanup    func()
}

// Apply imprints the workspace's settings onto a copy of opts so the driver
// can hand opts to the session/loop layer without further glue. Callers
// that need additional per-workspace tweaks (alias, ClusterID) layer them
// on top of the result.
func (w *Workspace) Apply(opts ProviderOpts) ProviderOpts {
	out := opts
	if w == nil {
		return out
	}
	if len(w.MCPCommand) > 0 {
		out.MCPCommand = w.MCPCommand
	}
	return out
}

// Executor provisions Workspaces. One Executor instance produces one or
// more independent workspaces depending on the driver's needs:
//   - DispatchSingle calls Provision once (single session)
//   - DispatchLoop with concurrency=N calls Provision N times (parallel
//     loop slots, each its own workspace)
//
// Implementations must be safe for sequential Provision calls; parallel
// invocation is not required (drivers sequence calls themselves).
type Executor interface {
	Name() string
	Provision(ctx context.Context, opts ProviderOpts, slotIdx int) (*Workspace, error)
}

// HostExecutor runs the session in the operator's current working
// directory with no per-session MCP override. The "no-op" executor:
// Provision returns a workspace with empty MCPCommand and a no-op
// Cleanup. This is the historical `--container=local` path.
type HostExecutor struct{}

func (HostExecutor) Name() string { return "host" }

func (HostExecutor) Provision(_ context.Context, _ ProviderOpts, _ int) (*Workspace, error) {
	return &Workspace{Name: "host", Cleanup: func() {}}, nil
}

// ContainerExecutor provisions slot containers via the existing
// `provisionSlot` primitive. One ContainerExecutor instance is shared
// across N parallel slots (single image build + cwd lookup + env
// collection happens once); each Provision call materializes one slot
// container + worktree under ~/.zdx/projects/<slug>/slots/<alias>-<idx>.
type ContainerExecutor struct {
	providerName string
	imageTag     string
	cwd          string
	agentCfg     config.AgentConfig
	envPairs     []string
}

// NewContainerExecutor builds the docker image, resolves cwd, and
// collects forwarded env vars — the per-orchestrator setup that was
// duplicated between runMCPContainerLoop and runMCPContainerSingle
// before this refactor.
func NewContainerExecutor(providerName string, opts ProviderOpts) (*ContainerExecutor, error) {
	if opts.RC.slug == "" {
		return nil, fmt.Errorf("--container=docker requires a project config with a remote slug")
	}
	agentCfg := opts.AgentCfg
	if agentCfg.ContainerMemory == "" {
		agentCfg.ContainerMemory = "4g"
	}
	if agentCfg.ContainerCPUs == "" {
		agentCfg.ContainerCPUs = "2"
	}

	imageTag, err := buildDevImage()
	if err != nil {
		return nil, err
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getwd: %w", err)
	}
	envPairs := collectContainerEnv([]string{"ANTHROPIC_API_KEY", "DATABASE_URL", "NO_COLOR"})

	// Reap any orphan slot containers whose names match this alias prefix.
	// A self-update re-exec replaces the host-side loop without
	// docker-stopping the slot containers; they linger as orphans and the
	// new process trips on "container name in use" when it tries to
	// recreate them. Reaping at startup recovers the cluster.
	reapOrphanSlotContainers(providerName, opts.Alias)

	fmt.Printf("[%s] container executor: provider=%s image=%s memory=%s cpus=%s\n",
		nowRFC3339(), providerName, imageTag, agentCfg.ContainerMemory, agentCfg.ContainerCPUs)

	return &ContainerExecutor{
		providerName: providerName,
		imageTag:     imageTag,
		cwd:          cwd,
		agentCfg:     agentCfg,
		envPairs:     envPairs,
	}, nil
}

func (e *ContainerExecutor) Name() string { return "container" }

func (e *ContainerExecutor) Provision(ctx context.Context, opts ProviderOpts, slotIdx int) (*Workspace, error) {
	s, err := provisionSlot(ctx, e.providerName, e.imageTag, e.cwd, opts, slotIdx, e.agentCfg, e.envPairs)
	if err != nil {
		return nil, err
	}
	return &Workspace{
		Name:       s.Name,
		MCPCommand: s.MCPCmd,
		Cleanup:    s.Cleanup,
	}, nil
}

// PickExecutor chooses an executor based on the resolved --container
// mode (canonical form from NormalizeContainer: "docker" or "local").
// Centralizes the host-vs-container decision so every entry point
// (bare `dx agent`, `dx agent loop`, `dx agent connect`) routes through
// the same logic instead of duplicating the branch.
func PickExecutor(mode, providerName string, opts ProviderOpts) (Executor, error) {
	if mode == ContainerDocker {
		return NewContainerExecutor(providerName, opts)
	}
	return HostExecutor{}, nil
}
