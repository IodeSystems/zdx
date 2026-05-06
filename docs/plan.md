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
| 2 — UI panel + pin/unpin + flag fix | ✅ shipped 2026-05-06 — API smoke + Playwright e2e (`TestDemoBrowser_AgentsPoolPanel`) cover the full pin/unpin loop |
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

  **UI walkthrough automated — 2026-05-06.** Replaced the manual
  Chrome-driven walkthrough with `TestDemoBrowser_AgentsPoolPanel`
  (`test/e2e/demo_browser_agents_pool_test.go`). Seeds a project-scoped
  agent via `/api/agents/register` and a global-pool agent by direct
  SQL insert (only WS handshake produces globals; the demo
  short-circuits with a row that has `originally_global=true,
  project_id=NULL`), then drives Playwright through pin → select
  project → confirm → unpin and asserts the scope chip /
  scope-immutable caption flip. Recorded as a `.webm` under
  `.zdx/demo/video/` like every other browser demo.

  Two harness gaps surfaced and were fixed inline so this demo (and
  every existing `requiresUI(t)` demo) can actually run:
  - `internal/devserver/devserver.go` now reads `STATIC_DIR` instead of
    hardcoding `""`. Without it, the ephemeral devserver returned
    `404 page not found` on every UI route — so the existing
    `TestDemoBrowser_ReleaseIndexGroupsByVersion` and
    `TestDemoBrowser_ProjectDirectionTab` were silently broken too.
    Run with `STATIC_DIR="$PWD/ui/dist"` after `pnpm build`.
  - `test/e2e/main_test.go` propagates `TEST_DATABASE_URL` into
    `srv.DSN` when running under `dx test`'s ephemeral mode, so DSN-
    requiring demos no longer skip when invoked through the harness.

  Selectors note (for future browser demos): MUI's Tooltip wraps each
  `<IconButton>` in a `<span>` with `aria-label="<title>"` — the button
  itself has no accessible name, and the prod build strips
  `data-testid` on Material icons. Locate by
  `span[aria-label="…"] button`, not by `GetByRole("button", {Name: …})`
  or `svg[data-testid="…Icon"]`.

## Phase 3 — mark-priority, cross-project queue, admin auth

Global agents can pick up cross-project work without manual pinning;
operator escalates urgent work via mark-as-priority; auth story is proper.

- [x] **Priority push** — operator escalation as an integer push, not a
  separate flag column. `UpsertTodo` now uses
  `priority = LEAST(zdx_todos.priority, EXCLUDED.priority)` so the bump
  survives re-evaluate (the natural priority computed for each kind
  becomes a ceiling — once an operator pushes lower, the lower value
  stays). Endpoint: `PUT /api/dx/projects/{slug}/todos/{key}/priority`
  body `{priority}`. UI verb: ↑ Push button on the todo detail page
  next to the priority chip; opens a small dialog with a number input.
  Adding the push verb to the queue-row and per-issue todo lists is a
  follow-up.
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

## Post-phase-2 backlog

Phase 2 shipped clean. With stabilization mode over, these can move
into the tracker (or be picked up directly from this doc — pick one
discipline and stick to it).

- **Workspace relocation** to `~/.zdx/workspaces/<project>/...`. Big
  refactor across agent provisioning, ship hooks, dx config lookup,
  dx-agent `--mcp-root`. The next significant pass.
- **Browser-demo cross-test flake.** `TestDemoBrowser_ProjectDirectionTab`
  passes alone but times out on the Goals heading when run after
  another browser demo in the same process. Pre-rewrite tests had the
  same binary-level isolation, so this likely predates phase 2 — but
  was masked by the universal SPA-404 regression. Suspected: the
  `.zdx/demo/video/` directory accumulating per context, or the
  shared playwright-go runtime not actually disposing between tests.
  Repro: `STATIC_DIR=… TEST_DRIVER=ui dx test --layer demo --filter
  TestDemoBrowser_`. Not blocking; the demos are individually
  reliable and demo runs are rarely batched.
- **Project priority schema** — prerequisite for phase 3's cross-project
  claim. Splits naturally: schema first (cheap), cache strategy second
  (deferred until queue size + access pattern data).
- **Mark-as-priority UI verb** — phase 3 element but useful even before
  phase 3 lands (boosts a todo for project-scoped agents too).
- **Cached cross-project queue view** — costing pass on cache-invalidation
  patterns required before implementation.

## Harness-fix knock-on findings (2026-05-06)

The `STATIC_DIR` and `srv.DSN` fixes that landed with
`TestDemoBrowser_AgentsPoolPanel` revived the rest of the
`requiresUI(t)` demo set, which had been failing or skipping silently
under `dx test --layer demo`:

- ✅ **`TestDemoBrowser_ReleaseIndexGroupsByVersion`** — was timing out
  on the SPA fallback 404; now passes.
- ✅ **`TestDemoBrowser_IssueFlow`** — was passing already (only hits
  `/api/health`); kept green by the harness changes.
- ✅ **`TestDemoBrowser_ProjectDirectionTab`** — passes solo after the
  panic fix (`a669d3ab`) and the Goals-only rewrite. Two layered
  problems closed:
  - playwright-go v0.5700.1 nil-derefs `*options[0].Exact` when
    `PageGetByTextOptions{}` is passed empty; drop the struct.
  - the test asserted a Constraints section and exercised Constraints
    CRUD, both removed in IS-627. Test rewritten Goals-only; goal
    Edit / Delete buttons scoped to the goal card (the IconButtons
    have no aria-label or data-testid, so a global `button`-index
    selector silently mismatches once the nav shell adds buttons).
  - **Flake when run in the same process after another browser
    demo:** `Goals` heading times out at 15s. Passes alone in 3s.
    Likely shared-playwright/resource contention — captured in the
    backlog below; not a blocker for the e2e fix landing.

## Pickup checklist for the next session

### Local dev env recap (already standing on 2026-05-06)

- Postgres in `tmp-postgres` container, host port **7601** (not 7650 —
  `dx daemon start` default; the earlier "7650" reference in this doc
  is from the prod-DB convention).
- dx-server on **7600**, vite on **7610** (loopback `::1`).
- Local admin token bootstrapped via `POST /api/setup/bootstrap` and
  promoted to `role='admin'` via SQL. User: `smoke@local`. Project
  `smoke` (id=1) created via `POST /api/project`.

### How to run the regression

The pin/unpin walkthrough now lives in
`TestDemoBrowser_AgentsPoolPanel`. Reproduction:

```bash
make build                   # picks up devserver STATIC_DIR change
cd ui && pnpm build          # produces ui/dist for the SPA fallback
cd ..
./bin/dx test e2e build      # rebuilds bin/zdx-test
STATIC_DIR="$PWD/ui/dist" TEST_DRIVER=ui \
    ./bin/dx test --layer demo --filter AgentsPool
```

Expected: `1 passed 0 failed 0 skipped` for the e2e adapter; vitest
adapter still fails 6 preexisting environment-card tests
(`useNavigate is not a function` — pre-existing, unrelated).

### After UI confirms

Phase 2 ships clean. Exit stabilization mode for GAPD: file new work
as tracker issues again, let the agent loop resume on this stream.
Then schedule workspace relocation, then phase 3.
