# GAPD — Global Agent Pool Design — CLOSED 2026-05-06

This stream shipped. Phases 1–3 plus the workspace-relocation
followup are in main. **Do not start work in this file.** If you
need to find something that lived here, `git log` is the source
of truth.

## Workspace relocation — done

**The directive:** every agent workspace, global AND
project-scoped, lives under the operator's home directory
(`~/.zdx/projects/<slug>/...`), not inside the operator's
project tree. Pinning a global to a project is metadata, not
a filesystem move.

**What was already correct:** the srcless / global-pool flow
(`internal/cli/agent/srcless.go`) already used
`~/.zdx/projects/<slug>/main` for the clone and
`~/.zdx/projects/<slug>/worktrees/<sid>` for per-session
worktrees. Default `WorkDir` for `GlobalAgentConfig` is
`~/.zdx/projects/`. No change needed there.

**What was wrong:** the container-slot path
(`internal/cli/agent/mcp_container.go`, used by
`dx agent loop --container=docker` for parallel
docker-isolated slots) put per-slot worktrees at
`cwd/.zdx/agent/slots/<alias>-<i>` inside the operator's
project root. Fixed: `slotWorktree` now gains a `slug` parameter
and computes `~/.zdx/projects/<slug>/slots/<alias>-<i>`. Same
git-worktree mechanics — `git worktree add` rooted at `cwd`
(operator's repo, where the parent loop process is launched
from), absolute path output, the `-v /proj/.git:/proj/.git`
bind-mount still resolves the gitdir-pointer file inside the
slot.

**Operator migration:** if you've run
`dx agent loop --container=docker` on this repo before, you may
have a stale `.zdx/agent/` directory in your project root. Delete
it — the daemon will recreate slots under
`~/.zdx/projects/<slug>/slots/` on next run.

## What shipped

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
