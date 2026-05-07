# GAPD — Global Agent Pool Design + Implementation

Single source of truth for the global-agent-pool stream. Last refresh 2026-05-06.

**Where this stream stands:** phases 1–3 effectively closed. One blocked
follow-up remains (admin-auth strict-reject) and it cannot land until
prod telemetry shows zero `agent connect: DEPRECATED:` log lines for a
release cycle. **Pick something else** in the meantime — the next
session should start by opening a different stream's plan, not this one.
The blocked follow-up plus a small UI "maybe" are listed under
[Pickup](#pickup--next-session) for completeness, but neither is ready
to start today.

**Why stabilization is over:** the agent loop, worktree provisioning,
codegen drift checks, and merge-train were co-evolving when this stream
opened. They are now stable enough that ordinary worker mode (claim
todo → branch → merge-train) is safe again on agent-side work. Use the
worker contract (`./bin/dx commit --intent`).

## Status

| Phase | State |
|-------|-------|
| 1 — schema + connect verb + list endpoint | ✅ shipped (`21bc378f`) |
| 2 — UI panel + pin/unpin + flag fix | ✅ shipped 2026-05-06 — API smoke + Playwright e2e (`TestDemoBrowser_AgentsPoolPanel`) cover the full pin/unpin loop |
| 3 — priority bump + cross-project queue + admin auth | 🟢 effectively closed — bump-spread, transitional admin auth, periodic reaper, `/api/dx/solo/claim-any`, and daemon migration shipped. Strict-reject flip on global auth held until prod telemetry is quiet. |

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

### Cross-project priority (phase 3 — landed without cache)

Project-priority × todo-priority composite score. Global agents pick the
highest composite item across every project they have access to. The
costing pass (prod: 4 projects, 1 active, 0 globals ever, 12/hr avg
claims) showed no cache layer was warranted. Implementation went
straight onto the live tables: `ClaimNextTodoAny` is a single CTE-UPDATE
ordered `p.priority, t.priority, t.created_at` with `FOR UPDATE OF t
SKIP LOCKED`. Revisit caching only if prod scales past ~50 projects
with concurrent globals.

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

### Authentication (phase 3 — transitional)

`dx agent connect --global` should use a server-admin token, not a project
API key. Phase 3 wired the admin-token path through `authorizeAgentRegister`
in `internal/server/handlers/handlers_agent_conn.go`:
- **Global registration with admin role + unscoped key** → silent allow.
- **Global registration with anything else** → allow + `agent connect:
  DEPRECATED: …` log line spelling out the role and scope so operators
  can find the callsite.
- **Project-scoped registration with a token whose `project_scope` does
  not include the slug** → hard reject with `StatusPolicyViolation`.

Strict-reject for the deprecated global path is held until prod
telemetry shows zero deprecation log lines for a release cycle (see
[Pickup](#pickup--next-session)).

### Heartbeat / liveness (phase 3 — server-driven reaper)

`last_heartbeat` is the source of truth. The WS connection is the
real-time liveness signal; the column persists last-known-alive so the UI
shows "online 4m ago" for agents that disconnected without clean shutdown.
Cleanup runs as a server-driven goroutine (`StartReaper` in
`internal/server/server.go`) — wakes every 1m, deletes rows with
`last_heartbeat < NOW() - 5m`. The original "separate admin-triggered
global reap" idea was abandoned: `ReapStaleAgents` was already a
single global DELETE with no project filter, so a per-project sweep
never existed. The manual `POST /api/agents/reap` endpoint and
`dx agent reap` CLI are gone (`123a71f2`); the periodic loop is the
only reap path now.

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

- [x] **Priority bump** — operator escalation as an integer bump, not a
  separate flag column. `UpsertTodo` now uses
  `priority = LEAST(zdx_todos.priority, EXCLUDED.priority)` so the bump
  survives re-evaluate (the natural priority computed for each kind
  becomes a ceiling — once an operator bumps lower, the lower value
  stays). Endpoint: `PUT /api/dx/projects/{slug}/todos/{key}/priority`
  body `{priority}`. UI verb: ↑ Bump button on the todo detail page
  next to the priority chip; opens a small dialog with a number input.
  (Originally landed as "Push" in `2625eb18`; renamed to "Bump"
  alongside the QueueView/Timeline spread to avoid the agent-dispatch
  collision — see "No push semantics" in the locked-in design.)
- [x] **Project priority schema** (`686633f9`) — `zdx_projects.priority
  INT NOT NULL DEFAULT 5` (1=highest, 9=lowest convention; not enforced
  at the DB layer). `/api/projects` payload carries it; `PUT
  /api/admin/project-priority` sets it. Consumer (cross-project claim)
  still gated on the costing pass below.
- [x] **Bump verb on queue rows + per-issue todos.** Done. Renamed
  the existing TodoDetail "Push" surface to **Bump** (terminology
  collision: "push" reads as agent dispatch, which we explicitly do
  not build — the mechanism is just lowering the priority integer).
  Spread the bump control to:
  - `QueueView` `TodoRow` (`ui/src/components/QueueView.tsx`) — chip
    `pN` + ↑ icon on every queue item, primary card and upcoming list.
  - `UnifiedTimeline` `TodoRow` (`ui/src/components/UnifiedTimeline.tsx`)
    — chip + ↑ on `created` events for unresolved todos only. The
    issue page renders its todos as timeline events rather than as a
    flat list, so this is the natural surface; if a flat
    issue-page todo list is wanted later, gate it as a separate task.
  Hook re-used: `useSetTodoPriority`. Dialog copy says "Bump" and
  "the bump sticks" (LEAST-preserved on re-evaluate).
- [x] **Cross-project queue browsing for unpinned global agents.**
  Done. Costing pass first: prod (migration 148, 4 projects, 1
  active, 0 globals ever, 12/hr avg claims, 168/hr peak — all
  single-project) does not warrant any caching layer. No
  `zdx_solo_global_view`, no materialized view, no triggers.
  Composite ordering pushed straight to the live tables:
  - New SQL `ClaimNextTodoAny` (queries/todos.sql) — single
    atomic CTE-UPDATE ordered by `p.priority, t.priority,
    t.created_at` with `FOR UPDATE OF t SKIP LOCKED`. Returns
    `project_slug` so the caller can route follow-ups.
  - New SQL `ListProjectsByPriority` (queries/projects.sql) —
    used by the new endpoint to refresh persisted queues in
    priority order before the atomic claim, mirroring today's
    per-project /claim invariant (stale persisted state can't
    hide claimable work).
  - New endpoint `POST /api/dx/solo/claim-any`
    (handlers_solo.go) body `{agent_id, lease_minutes, mode}`.
    Refreshes every project's queue, then ClaimNextTodoAny;
    404 when the union is empty.
  - Existing `POST /api/dx/solo/claim` with `slug=""` retains
    its first-project-with-anything-claimable behavior so
    older daemons keep working — the new endpoint is
    additive. Daemon migration to claim-any is a follow-up.
  Smoke-tested against the local 7601 dev DB: claim-any on a
  project with no real work produced a synthetic
  owner:goals health todo with `project_slug: "smoke"`,
  status 200; with no projects-with-claimable-work the
  endpoint would 404. With many globals colliding on the
  same row, SKIP LOCKED hands them different items (same
  guarantee as ClaimNextTodo). When prod scales past ~50
  projects with concurrent globals, revisit caching against
  measured numbers — the costing pass note above gives the
  thresholds to watch.
- [x] **Daemon migration onto `/claim-any`.** Done.
  `claimNextTodo` (`internal/cli/agent/claim.go`) now branches
  on `rc.slug`: empty (global / srcless / `DX_GLOBAL=1`) goes
  to `POST /api/dx/solo/claim-any` without a `slug` field;
  non-empty stays on `POST /api/dx/solo/claim` exactly like
  before. The wire response shape is identical so the
  downstream lifecycle (`take.go`'s clone+init, lease renew,
  release/resolve) keeps working unchanged — global daemons
  just pick the composite-priority winner instead of the
  first-project-with-anything-claimable. Smoke-tested
  side-by-side against the local DB: both endpoints return
  HTTP 200 with the right `project_slug`. Still raw `http`
  rather than dxclient — switching to typed `SoloClaimAny`
  is a separate raw-api-calls cleanup.
- [x] **Admin token auth — transitional.** Done. WS handshake now
  authorizes the registration against the token's role +
  `project_scope` (already stamped on ctx by `apiKeyMiddleware`).
  Logic lives in `authorizeAgentRegister`
  (`internal/server/handlers/handlers_agent_conn.go`):
  - **Global** (`project_slug==""`) with role=admin AND unscoped →
    silent allow. Anything else → allow + DEPRECATED log line. The
    log spells out role and whether the key is scoped, so operators
    can find their `dx agent connect --global` callsites and migrate
    them to admin tokens before the strict-reject flip.
  - **Project-scoped** (`project_slug!=""`) when the token has
    a non-empty `project_scope` that does NOT include the slug →
    hard reject with `StatusPolicyViolation` + reason
    `"token not in project scope"`. This was already a defense-in-
    depth gap (any project token could register an agent under
    any other project) and is closed here. Empty scope (admin or
    generic CLI key) is unrestricted.
  Tests: `TestAuthorizeAgentRegister` covers all 9 quadrants
  (admin/non-admin × scoped/unscoped × global/in-scope/out-of-scope
  registration). Strict-reject for the global path is the follow-up
  — flip when telemetry shows no remaining deprecation log lines
  in prod.
- [ ] **Push-work commands: explicitly NOT BUILDING.** The priority-bump
  mechanism replaces dispatch.
- [x] **Periodic in-server reaper** (replaces dedicated reap idea).
  Done. The original GAPD concern was "a slow heartbeat doesn't get
  an agent reaped by an unrelated project sweep" — but reading the
  code revealed `ReapStaleAgents` was already a single global DELETE
  with no project filter, AND `/api/agents/reap` wasn't admin-gated
  AND nothing called it on a timer. Operators were running
  `dx agent reap` by hand (when they remembered to). Replaced the
  whole surface with a server-driven goroutine in `StartReaper`
  (`internal/server/server.go`): wakes every 1m, deletes rows with
  `last_heartbeat < NOW() - 5m`. Wired from `cmd/dx-server/main.go`
  alongside `StartBudgetWatcher` / `StartTaskRecovery`. Removed
  `POST /api/agents/reap` (handlers_agents.go) and `dx agent reap`
  CLI (cli/agent/agent.go); regenerated openapi.json. The
  global-vs-project split became moot once cleanup is server-driven.

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

## Pickup — next session

This stream is **not the right place to start work today.** Both
remaining items are cold:

1. 🚫 **Strict-reject the deprecated global-auth path.** Blocked.
   `authorizeAgentRegister`
   (`internal/server/handlers/handlers_agent_conn.go`) currently logs
   `agent connect: DEPRECATED: …` and accepts globals registered with
   non-admin or project-scoped tokens. Flip the global branch to
   return `"requires admin token"` once **prod telemetry shows zero
   such lines for a full release cycle.** That's a one-line change in
   the helper plus flipping the matching cases in
   `TestAuthorizeAgentRegister` from `wantDeprecated: true` to
   `wantReject: true`. Do not flip until prod logs are quiet — flipping
   too early breaks every operator's `dx agent connect --global`
   workflow simultaneously.

2. 💤 **(maybe) Flat issue-page todo list.** Bump now lives on the
   `UnifiedTimeline` `created` event for each issue's todos. If an
   operator asks for a flat actionable list (parallel to the existing
   "Tasks" section in `IssueDetail`), add a small "Open todos" section
   above `BlockerQuestionsSection` and put the bump there too. **No
   one has asked.** Don't volunteer it.

If neither is ready, open another stream's `plan/plan.md` (the
project-wide roadmap) and pick the next item there — GAPD is paged out.

## Out-of-band follow-ups (not part of GAPD, but discovered here)

These were uncovered while landing phase 3 and don't belong to a
specific phase. Each is small enough to grab as a one-off. None
blocks the GAPD stream from being considered done.

- **Workspace relocation** to `~/.zdx/workspaces/<project>/...`. Big
  refactor across agent provisioning, ship hooks, dx config lookup,
  and dx-agent's `--mcp-root`. Decouples agent worktrees from the
  operator's project tree, fixing the operator-vs-agent collision
  class at the directory level rather than per-slot. Sized days, not
  hours; worth a fresh design pass before starting.
- ✅ **Daemon claim raw-`http` swap (2026-05-06).**
  `internal/cli/agent/claim.go` now uses
  `cli.NewClient(rc.url, rc.key)` + the typed
  `SoloClaimWithResponse` / `SoloClaimAnyWithResponse` /
  `SoloRenewWithResponse` / `SoloReleaseWithResponse` calls. Wire
  shapes verified against `SoloClaimRequest` / `SoloClaimAnyRequest`
  / `SoloRenewRequest` / `SoloReleaseRequest`; the internal
  `claimedTodo` shape is preserved (all downstream callers in
  `take.go`, `manager.go`, `claude.go` keep working unchanged).
  Side-effect: outbound requests now also carry
  `X-ZDX-Agent-Id` / `X-ZDX-Session-Id` attribution headers (via
  `cli.Client`'s authEditor) when those env vars are set — old
  raw-http path didn't. Harmless additive change. `go vet` +
  `go test ./internal/cli/agent/...` clean.

  **Plan-note correction:** the original followup claimed this
  swap would "clear the advisory `raw-api-calls` lint warning."
  It doesn't — the lint detects the `c.Get("/api/`, `c.Post("/api/`,
  `c.Delete("/api/` patterns in `internal/cli/`, which `claim.go`
  never used (it was raw `http.NewRequest`). The 3 remaining lint
  hits are in different files and unaffected by this change. See
  next bullet.

- ✅ **`raw-api-calls` lint advisory cleared (2026-05-06).**
  All 3 remaining callsites converted to typed dxclient:
  - `internal/cli/configcmd/config.go` register flow now uses
    `c.ListProjectsWithResponse` + `c.CreateProjectWithResponse`
    (with `c.CheckStatus` for the authHint-enriched 4xx path).
  - `internal/cli/util.go` `(*Client).FetchClaimBase` now uses
    `c.SoloListClaimsWithResponse` against the typed
    `SoloListClaimsParams{Slug: …}` and walks the
    `*resp.JSON200.Todos` slice. `net/url` import retained — still
    used by `QuerySlug`.
  `bin/lint --intent` now reports `OK: raw-api-calls` (was
  `WARN: 3 callsites in 2 files`). `go vet` + `go test ./internal/cli/...`
  clean.
- **Browser-demo cross-test flake.**
  `TestDemoBrowser_ProjectDirectionTab` passes alone (~3s) but times
  out on the Goals heading when run after another browser demo in the
  same process. Pre-rewrite tests had the same binary-level isolation,
  so this likely predates phase 2 — it was masked by a universal
  SPA-404 regression. Suspected: `.zdx/demo/video/` accumulating per
  context, or shared playwright-go runtime not actually disposing
  between tests. Repro:
  `STATIC_DIR=… TEST_DRIVER=ui dx test --layer demo --filter TestDemoBrowser_`.
  Not blocking — demos are individually reliable and demo runs are
  rarely batched.
- ✅ **Jest environment-card failures fixed (2026-05-06).**
  6 tests in `ui/src/routes/project/$slug/environments/index.test.tsx`
  were failing with `useNavigate is not a function`. Cause: the
  `jest.mock('@tanstack/react-router', …)` factory only stubbed
  `createFileRoute` + `Link`. `EnvironmentCard` calls `useNavigate()`
  (added to drive a "View branches" navigation), so the mock fell
  through to `undefined` at runtime. Fix: add
  `useNavigate: () => jest.fn()` to the mock. Full ui suite now
  84/84 passing (was 78/84). The plan note called this "vitest"
  but the project uses jest — corrected.

## Reference

### Local dev env (verified end of session 2026-05-06)

- Postgres in `tmp-postgres` container, host port **7601**, DSN
  `postgres://zdx:zdx@127.0.0.1:7601/zdx?sslmode=disable`.
- Production dx-server pid is bound to **7600**; if you need a
  side-by-side instance, start one on `PORT=7699` so it doesn't
  collide. The session's smoke tests all hit 7699.
- Vite dev server (when run) on **7610**, loopback `::1`. `pnpm run dev`
  swallows `--host`; `server.host` in `vite.config.ts` is also ignored
  by vite v8. If you need LAN access, run
  `./node_modules/.bin/vite --host 0.0.0.0` directly.
- DB at migration **150** (project priority). Latest GAPD migrations
  are `148_agents_global_pool` and `149_agent_originally_global`.
- Bootstrap user: `smoke@local`, role `admin`. Project `smoke`
  (id=1) created via `POST /api/project`. Token in
  `zdx_api_keys ORDER BY created_at DESC LIMIT 1`.
- Periodic reaper (`StartReaper`) deletes any agent row with
  `last_heartbeat < NOW() - 5m` every 1m; smoke-test daemons left
  running in stale state will be gone before the next session
  starts. No manual cleanup needed.

### How to run the demo regression

```bash
make build                   # picks up devserver STATIC_DIR + DSN propagation
cd ui && pnpm build          # produces ui/dist for the SPA fallback
cd ..
./bin/dx test e2e build      # rebuilds bin/zdx-test
STATIC_DIR="$PWD/ui/dist" TEST_DRIVER=ui \
    ./bin/dx test --layer demo --filter AgentsPool
```

Expected: `1 passed 0 failed 0 skipped` for the e2e adapter. Vitest
adapter fails 6 preexisting environment-card tests (see out-of-band
follow-ups).

### Worker-contract reminder

Workers commit intent only. Do not stage `internal/db/*.sql.go`,
`internal/dxclient/models.gen.go`, `ui/src/api.gen.ts`,
`schema/shipped.sql`, `internal/db/models.go`, or `ui/src/routeTree.gen.ts`.
Use `./bin/dx commit --intent` — it inspects the staged set, warns on
each generated file, unstages it, then commits the remainder.

`internal/dxclient/openapi.json` IS intent (committed by author when
handlers change the OpenAPI shape). The dev-server regenerates it on
boot — start `bin/dx-server` briefly after handler edits to refresh
both `openapi.json` and the downstream gen files, then commit
openapi.json with the rest of your intent.

### Phase-3 commits (2026-05-06)

The order is the order they landed; later commits assume earlier
ones in the same chain.

- `91f5c0af` style(server): gofmt agents handler struct alignment
- `cd7b09be` feat(ui): rename Push → Bump and spread to QueueView + issue timeline
- `cbe187e7` feat(server): admin-token auth for global agent connect (transitional)
- `ae5d2045` fix(devmode): write api.gen.ts to absolute path (avoid ui/ui ghost dir)
- `123a71f2` feat(server): periodic reaper, drop manual /api/agents/reap + CLI
- `c3313b43` feat(server): /api/dx/solo/claim-any cross-project todo claim
- `4dc6ded0` feat(agent): route empty-slug daemon claims to /claim-any

Earlier phase-3 commits (prior session):

- `b1a12266` test(e2e): automate GAPD phase-2 UI walkthrough
- `a669d3ab` test(e2e): drop empty PageGetByTextOptions to avoid library nil-deref
- `66fa83e7` test(e2e): rewrite direction-tab demo Goals-only after IS-627
- `686633f9` feat(projects): priority column for cross-project agent claim ordering
- `2625eb18` feat(todos): operator priority-push as integer (LEAST-preserved)
- `21bc378f` feat: phase 1 (schema + connect verb + list endpoint)
