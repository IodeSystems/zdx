# GAPD — Global Agent Pool Design + Implementation

Single source of truth for the global-agent-pool stream. Captured 2026-05-06,
updated as decisions firm up.

**Stabilization mode (active).** This work is driven by direct human edits.
No `dx issue add` / `dx todo` / `dx agent claude --loop` on GAPD itself
until phase 2 ships clean. Anything that would normally become a tracker
issue lives inline below. Ignore `plan/plan.md` for this stream — that's the
broader product roadmap, orthogonal to GAPD.

## Status

| Phase | State |
|-------|-------|
| 1 — schema + connect verb + list endpoint | ✅ shipped (`21bc378f`) |
| 2 — UI panel + pin/unpin + flag fix | API smoke ✅ 2026-05-06; UI render verification pending (vite is loopback-only) |
| 3 — mark-priority + cross-project queue + admin auth | design only |

## Why stabilization mode

The agent loop, worktree provisioning, codegen drift checks, and merge-train
are co-evolving. Self-hosting GAPD on the agent system right now means
fighting the system we're stabilizing — codegen races, slot collisions,
merge-train rewrites of in-flight branches. Until phase 2 lands cleanly,
GAPD work goes direct: human → branch → PR → dev. The agent loop can
resume on this stream once it is itself stable.

## Goal

Run `dx agent connect --global` and have it register into a server-wide
agent pool, visible in a top-level `/agents` nav item, controllable from
the Web UI. Project-scoped agents continue to work and also appear in the
same panel — "global" is just one possible scope value.

## Locked-in design decisions

These aren't negotiable without a fresh round of discussion.

### Agent lifecycle

- **`dx agent connect`** is the primary verb. Registers with the server via
  the existing WS handshake and either:
  - **starts the work loop immediately** (default), or
  - **stays idle** with `--idle`, holding only the WS connection so the
    operator can pause/resume/drain/dispatch from the UI.
- **`--global` is a flag on `connect` itself, not on the parent.** Order:
  `dx agent connect --global` (NOT `dx agent --global connect`). Reasoning:
  global is a connection-manager concern, not part of the agent's identity.
  Phase 1 currently has it as a parent persistent flag — fix is the first
  item in phase 2.
- **Project-scoped agents are scope-immutable.** Re-register if you want to
  change scope. Less footgun surface.
- **Global agents can be pinned to a project** via assign/unassign. The
  agent stays visible in the global pool while pinned — pinning expresses
  preference, not a scope conversion.

### Behavior of an unassigned global agent

Idle by default, until either pinned to a project (claims that project's
queue) or the operator promotes a todo to high priority somewhere it can
see (cross-project priority — phase 3).

### Cross-project work for assigned global agents

Once pinned to a project, the agent claims from that project's queue
exactly like a project-scoped agent.

### Cross-project priority (phase 3)

Project-priority × todo-priority composite score. Global browsers pick the
highest composite item across every project they have access to.
Implementation needs a server-wide cached priority list (re-scanning every
project's queue on every claim is too expensive). Defer until we have a
cost model for cache invalidation.

### Operator → agent dispatch (phase 3)

**No push semantics.** Operator does not send "agent X work issue Y".
Instead, **mark-as-priority** boosts the todo's queue position; the agent's
normal claim path picks it up next iteration. One mechanism (queue) rather
than two (queue + push).

### Workspace relocation (deferred — significant)

All workspaces move to `~/.zdx/workspaces/<project>/...` — global-pool
agents and project-scoped agents alike. Today: `./.zdx/agent/slots/<alias>/`
lives inside the operator's project root, which is the source of the
`git worktree` + bind-mount complexity in `mcp_container.go`. New layout
decouples the agent's working tree from the operator's project tree by
default, fixing the operator-vs-agent collision class at the directory
level rather than per-slot.

Touches: agent provisioning, `bin/ship`'s view of the workspace, dx-config
lookup chain, dx-agent MCP server's `--mcp-root`. Not a phase 2 blocker —
phase 2 ships against the current layout. Schedule a focused pass after
phase 2.

### Authentication

`dx agent connect --global` should use a server-admin token, not a project
API key. Today's WS auth is `?api_key=…` (project token) which technically
works for the WS upgrade but is wrong semantically. Phase 3 wires the
admin-token path properly. Until then: operators with admin-tier API keys
use those for global registration.

### Heartbeat / liveness

`last_heartbeat` is the source of truth. The WS connection is the
real-time liveness signal; the column persists last-known-alive so the UI
shows "online 4m ago" for agents that disconnected without clean shutdown.
**Reap** (`dx agent reap`) deletes rows older than threshold. Today it
sweeps per-project. For globals: separate admin-triggered reap so a slow
heartbeat doesn't get an agent reaped by an unrelated project sweep.

### Web UI is the primary control surface

Everything except the initial `connect` happens via WebUI: pause, resume,
drain, assign, unassign, mark-as-priority. CLI commands for these actions
remain (don't remove them) but aren't the operator's daily-driver path.

## Phase 1 (shipped — `21bc378f`)

- ✅ Schema migration `148_agents_global_pool`: `project_id` nullable,
  `idle BOOLEAN NOT NULL DEFAULT false`, partial index on global pool
- ✅ WS handshake (`/api/agents/connect`): `project_slug`, `idle` fields
- ✅ `agentdaemon.Daemon`, `agentconn.Conn`, `ProviderOpts` carry the new
  fields
- ✅ `dx agent connect [--idle]` verb (currently parent-flag bug — fixed
  in phase 2)
- ✅ `GET /api/agents` server-wide list endpoint (joins `zdx_projects.slug
  + name`)
- ✅ `AgentItem` extended: `ProjectID *int32`, `ProjectSlug`,
  `ProjectName`, `Idle`

## Phase 2 — UI panel + pin/unpin

Operator can see every connected agent in one place and pin/unpin global
agents to projects from the UI. Order matters: small fix → schema bit →
endpoints → UI.

- [x] **Flag relocation.** Move `--global` from parent persistent flag to
  `connect`-local. Done: `dx agent connect --global` registers in the global
  pool; `dx agent --global connect` errors as unknown flag.
  `loadAgentRuntime` now auto-detects srcless via `IsSrcless()` +
  `DX_GLOBAL` env override (no longer reads the flag). Followup recorded:
  operators with both project config and a `~/.zdx/config.yaml` who want
  to register globally using the *global* remote credentials must export
  `DX_GLOBAL=1` — previously `--global` did both. Acceptable per the
  design note "global is a connection-manager concern, not part of the
  agent's identity."
- [x] **`originally_global` bit + WS-handshake DB persistence.** Done.
  Migration `149_agent_originally_global` adds the column with backfill
  (true where `project_id IS NULL`). New `RegisterGlobalAgent` upsert
  query writes globals; `RegisterAgent` writes project-scoped. WS
  handshake (`HandleAgentConnect`) now upserts the row on each connect
  via the appropriate path. `originally_global` is set on first insert
  and never updated. `AgentItem.OriginallyGlobal` exposed via `/api/agents`.

  **Discovered gap (closed):** the phase 1 `dx agent connect` path never
  inserted rows into `zdx_agents` — only the legacy
  `/api/agents/register` REST endpoint did, and it required a project
  slug. So global agents existed only in the in-memory
  `agentconn.Registry` and `GET /api/agents` (DB-backed) silently missed
  them. Fixed here: handshake upserts both project-scoped and global
  rows.
- [x] **`POST /api/agents/{id}/assign`** body `{project_slug}`. Done.
  Returns 400 when `originally_global=false` (project-scoped agents are
  scope-immutable); 404 on missing agent or project; otherwise sets
  `project_id` to the named project. Returns the updated `AgentItem`
  with `project_slug + project_name` populated.
- [x] **`DELETE /api/agents/{id}/assign`**. Done. Returns 400 when
  `originally_global=false`; otherwise clears `project_id` back to NULL.
  SQL `AssignAgentToProject` and `UnassignAgent` queries both carry
  `WHERE id = $1 AND originally_global = true` as a defense-in-depth
  guard alongside the handler-level check.
- [x] **UI `/agents` route** + nav item. Done. New TanStack Router file
  route at `ui/src/routes/agents.tsx`, top-level. MUI table renders
  alias / scope / status / connected / last-heartbeat / pin / actions.
  Action buttons wire to `POST /api/agents/{id}/command`
  (pause/resume/drain) and the new assign/unassign endpoints.
  Auto-refresh via `useQuery({ refetchInterval: 5000 })` — no WS yet
  for v1; the panel is low-traffic. Hooks in `ui/src/api/index.ts`:
  `useAllAgents`, `useAssignAgent`, `useUnassignAgent`, `useAgentCommand`.
- [x] **UI nav placement.** Done. Entry placed beside `Activity` /
  `Admin` (top-level, not inside the per-project group).
- [x] **Smoke test (API path) — 2026-05-06.** Local dev DB on 7601 +
  dx-server on 7600. Migration 149 applied cleanly (originally_global
  column added, partial-index intact). Validated end-to-end via
  `dx agent connect --idle` (project-scoped + `--global`):
  - WS handshake persists rows on connect — closes the phase-1 gap.
  - Project-scoped row: `project_id=1, originally_global=false`; global
    row: `project_id=NULL, originally_global=true`.
  - `POST /api/agents/{id}/assign` pins global → project; rejects
    project-scoped with 400 "originally registered project-scoped".
  - `DELETE /api/agents/{id}/assign` unpins global → NULL; rejects
    project-scoped symmetrically.
  - `POST /api/agents/{id}/command` flips status active↔paused.
  - Disconnect on agent shutdown: connection_state goes to
    "disconnected", status retained.

  **UI render verification still pending.** The panel renders against
  the same `/api/agents` shape the API smoke confirmed, but the dev
  vite binds to `::1` only, so the UI route can't be exercised from a
  remote browser without an SSH tunnel. Pickup checklist below covers
  the remaining browser walkthrough.

## Phase 3 — mark-priority, cross-project queue, admin auth

Global agents can pick up cross-project work without manual pinning;
operator escalates urgent work via mark-as-priority; auth story is proper.

- [ ] **Mark as priority.** Per-todo flag (or priority-bump column) that
  overrides normal queue ordering. Surfaces the todo at the top of the
  next claim regardless of kind/priority. UI verb on the todo row + per-
  issue page.
- [ ] **Cross-project queue browsing for unpinned global agents.**
  - Schema: `zdx_projects.priority INT NOT NULL DEFAULT 5`
  - Server: cached cross-project priority list (`zdx_solo_global_view`,
    materialized view, or trigger-rebuilt cache — TBD after costing pass).
  - Claim path: global agents (no `project_id`, not pinned) call
    `POST /api/dx/solo/claim-any` against the cross-project view.
- [ ] **Admin token auth.** Server-admin token recognition for
  `dx agent connect --global`; reject project tokens for global
  registration (or accept both transitionally).
- [ ] **Push-work commands: explicitly NOT BUILDING.** The promotion
  mechanism replaces dispatch.
- [ ] **Dedicated reap for globals.** Admin-triggered, separate from
  per-project sweeps.

## Stabilization findings (resolved inline)

Things uncovered while landing phase 2; fixed here rather than filed
separately to keep the stream coherent.

- **`api.gen.ts` was stale on dev tip** — committed copy carried
  `/api/constraint*` paths that were removed in IS-627. The dev-server
  regen-on-startup keeps the consumer typed against ghost endpoints,
  hiding dead UI code. Surfaced when this stream's `make gen-dxclient`
  produced a fresh spec without the dead paths. Fix: deleted the
  `useConstraints / useCreate|Update|DeleteConstraint` hooks from
  `ui/src/api/index.ts`, removed the Constraints `Section` from
  `GoalsTab.tsx`, trimmed the constraint-only tests in `GoalsTab.test.tsx`.
  Doctor's `ConstraintCount` field stays — it reads constraints from
  the DB directly, unrelated to the removed REST surface.
- **WS handshake didn't persist agent rows** — see phase 2
  `originally_global` task above (closed).
- **`dx agent connect --idle` without `--alias` failed handshake.**
  `runIdleDaemon` (agent.go) called `startDaemon` without defaulting
  `opts.Alias`, so the WS payload sent `agent_id=""` and the server
  closed with `StatusProtocolError "invalid handshake"`. Surfaced
  during the 2026-05-06 smoke test. Defaulting was previously scoped
  to `RunManagedLoop` (manager.go:166-168). Fix: mirror the same
  default (`provider + "-" + uuid[:8]`) inside `runIdleDaemon` before
  the `startDaemon` call. Re-tested: `dx agent connect --provider=claude
  --idle` registers as `claude-<8hex>`.

## Deferred (kept in this doc — not filed as issues)

Stabilization mode: do not file these as tracker issues. Pull from this
list directly when the time comes.

- **Workspace relocation** to `~/.zdx/workspaces/<project>/...`. Big
  refactor across agent provisioning, ship hooks, dx config lookup,
  dx-agent `--mcp-root`. Schedule after phase 2.
- **Project priority schema** — prerequisite for phase 3's cross-project
  claim. Splits naturally: schema first (cheap), cache strategy second
  (deferred until queue size + access pattern data).
- **Mark-as-priority UI verb** — phase 3 element but useful even before
  phase 3 lands (boosts a todo for project-scoped agents too).
- **Cached cross-project queue view** — costing pass on cache-invalidation
  patterns required before implementation.

## Pickup checklist for the next session

Only the UI render walkthrough remains. API smoke validated end-to-end
on 2026-05-06 (see phase 2 smoke entry); only the browser-side render
of `/agents` is unverified, because dev vite binds loopback-only.

### Local dev env recap (already standing on 2026-05-06)

- Postgres in `tmp-postgres` container, host port **7601** (not 7650 —
  `dx daemon start` default; the earlier "7650" reference in this doc
  is from the prod-DB convention).
- dx-server on **7600**, vite on **7610** (loopback `::1`).
- Local admin token bootstrapped via `POST /api/setup/bootstrap` and
  promoted to `role='admin'` via SQL. User: `smoke@local`. Project
  `smoke` (id=1) created via `POST /api/project`.

### Remaining: UI walkthrough

1. SSH-tunnel vite from your laptop:
   `ssh -L 7610:localhost:7610 desktop` (or restart vite with
   `--host 0.0.0.0` / set `server.host` in `ui/vite.config.ts`).
2. Open `http://localhost:7610/agents` in your browser. First visit
   needs the token in `localStorage`:
   `localStorage.setItem('zdx_api_token', '<bootstrap-token>')`, then
   reload.
3. With both smoke agents running locally, confirm:
   - Two rows: `smoke-proj` (Scope=`smoke project`, no pin button —
     `scope-immutable` caption) and `smoke-glob` (Scope=`global`
     chip, pin button visible).
   - Pin `smoke-glob` from the UI: scope flips to `smoke project`,
     `pinned` chip appears. Unpin: scope returns to `global`.
   - Pause/resume/drain action buttons flip status as expected.

### After UI confirms

Exit stabilization mode for GAPD: file new work as tracker issues
again, let the agent loop resume on this stream. Then schedule
workspace relocation, then phase 3.
