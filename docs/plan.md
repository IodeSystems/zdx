# GAPD — Global Agent Pool Design

Phases 1–3 shipped. **One open item remains** before the stream
closes: workspace relocation (see [Open](#open) below). Until that
lands, a "global" agent isn't really global — it inherits the
operator's project tree the moment it pins.

## Open

### Workspace relocation — global-only workspace path

**Why it's GAPD, not a deferred cleanup:** today every agent
workspace lives under `./.zdx/agent/slots/<alias>/` inside the
operator's project root. That's tolerable for a project-scoped
agent that never moves, but it breaks the global-pool model: when
a global agent pins to project A, its workspace materializes
*inside* A's tree. Unpin and repin to B → either the workspace
follows (now inside B's tree, identity confused) or it stays in A
(stale). Either way, "global" is a lie at the filesystem layer.

**Design directive (locked-in this session):** all agent
workspaces — global AND project-scoped — live under
`~/.zdx/workspaces/<project>/...`. The operator's project tree
stops being an agent-workspace host entirely. There is no
per-project-tree path; pinning is a metadata flip, not a
filesystem move.

**Touches:**
- agent provisioning (`internal/cli/agent/manager.go`,
  `internal/cli/agent/take.go`'s clone+init path)
- `mcp_container.go` bind-mount layout
- dx-agent's `--mcp-root` resolution
- `bin/ship`'s view of where the workspace is rooted
- dx-config lookup chain (when `dx` runs from inside an agent
  workspace, how it finds the project's `.zdx/config.yaml`)
- migration story for any operator already running the current
  layout — probably "stop the daemon, blow away `.zdx/agent/`,
  restart": stateless workspaces are a feature

Sized days, not hours. Worth a fresh design pass — write
`plan/spike-gapd-workspace-relocation.md` before touching code.

## What shipped

- **Phase 1** — schema migration `148_agents_global_pool`, WS handshake
  carrying `project_slug`+`idle`, `dx agent connect [--idle]` verb,
  `GET /api/agents` server-wide list, `AgentItem` extended with project
  + global-pool fields. Single commit: `21bc378f`.
- **Phase 2** — `--global` flag relocated to `connect`, migration
  `149_agent_originally_global` + WS-handshake DB persistence,
  `POST/DELETE /api/agents/{id}/assign` with `originally_global`
  defense-in-depth, top-level `/agents` UI route with pin/unpin/
  pause/resume/drain action buttons, end-to-end Playwright demo
  (`TestDemoBrowser_AgentsPoolPanel`).
- **Phase 3** — operator priority bump (`PUT
  /api/dx/projects/{slug}/todos/{key}/priority`,
  `LEAST(zdx_todos.priority, EXCLUDED.priority)` in `UpsertTodo`),
  Bump UX spread to QueueView + UnifiedTimeline + IssueDetail,
  `zdx_projects.priority` schema, `/api/dx/solo/claim-any`
  cross-project claim with composite priority order, daemon
  migrated onto claim-any for global mode, periodic in-server
  reaper (`StartReaper` in `cmd/dx-server`), strict-reject for
  non-admin global-handshake auth.

## Locked-in design decisions (still load-bearing)

- **`dx agent connect` is the primary verb.** `--global` is a
  flag on `connect`, not on the parent. Project-scoped agents are
  scope-immutable; global agents can be pinned/unpinned without
  changing scope.
- **Workspace location is global-only** (see [Open](#open)). All
  agent workspaces live under `~/.zdx/workspaces/<project>/...`
  regardless of the agent's scope. Pinning is metadata, not
  filesystem.
- **No push semantics.** Operator escalation goes through priority
  bump; the agent's normal claim path picks the highest composite
  priority. One mechanism, not two.
- **`last_heartbeat` is the source of truth for liveness;** the WS
  connection is the real-time signal. Periodic reaper (1m wake,
  5m stale threshold) lives in the server, not on a manual
  endpoint or CLI.
- **Web UI is the primary control surface** for everything except
  the initial `connect` call.
- **Worker contract.** Workers commit intent only — don't stage
  `internal/db/*.sql.go`, `internal/dxclient/models.gen.go`,
  `ui/src/api.gen.ts`, `schema/shipped.sql`,
  `internal/db/models.go`, or `ui/src/routeTree.gen.ts`. Use
  `bin/dx commit --intent`. The merge-train owns regen.
- **Priority is ascending** (lower number = more urgent). Same
  convention everywhere: `zdx_projects.priority` (1=highest,
  9=lowest), todo priorities, claim ORDER BY, the bump endpoint
  copy. `LEAST(existing, new)` in `UpsertTodo` reads as "the
  more-urgent value wins."

## Commit chain

Phase-1:
- `21bc378f` feat: phase 1 (schema + connect verb + list endpoint)

Phase-2:
- `686633f9` feat(projects): priority column for cross-project agent claim ordering
- `66fa83e7` test(e2e): rewrite direction-tab demo Goals-only after IS-627
- `a669d3ab` test(e2e): drop empty PageGetByTextOptions to avoid library nil-deref
- `b1a12266` test(e2e): automate GAPD phase-2 UI walkthrough

Phase-3:
- `2625eb18` feat(todos): operator priority-push as integer (LEAST-preserved)
- `4dc6ded0` feat(agent): route empty-slug daemon claims to /claim-any
- `c3313b43` feat(server): /api/dx/solo/claim-any cross-project todo claim
- `123a71f2` feat(server): periodic reaper, drop manual /api/agents/reap + CLI
- `ae5d2045` fix(devmode): write api.gen.ts to absolute path
- `cbe187e7` feat(server): admin-token auth for global agent connect (transitional)
- `cd7b09be` feat(ui): rename Push → Bump and spread to QueueView + issue timeline
- `91f5c0af` style(server): gofmt agents handler struct alignment
- `200ad102` docs(plan): refresh GAPD handoff for clean cold-read
- `03682d45` feat(server): strict-reject deprecated global-auth path

Out-of-band cleanups landed alongside (not strictly GAPD):
- `6faef8ed` refactor(agent): typed dxclient for daemon claim/renew/release
- `52f37f3c` refactor(cli): typed dxclient for last 3 raw-api-call hits
- `f454f116` test(ui): add useNavigate to react-router mock in environments tests
- `e7d02d74` test(e2e): scope Direction demo locators to <main> to avoid drawer nav
- `a0fa0e01` test(e2e): use bump-style priority in evaluate-diff tests
- `f3f93cf5` feat(ui): flat 'Open todos' section on issue detail
