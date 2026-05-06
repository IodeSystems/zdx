# Global Agent Pool — Design + Implementation Plan

Captured 2026-05-06 during a multi-hour design conversation between Carl and
Claude. Phase 1 already shipped (commit `21bc378f`); phases 2/3 remain.

This doc is the source of truth for picking the work back up next session
without losing context. Updates land here as decisions firm up.

## Goal

Run `dx agent connect --global` and have it register into a server-wide
agent pool, visible in a top-level `/agents` nav item, controllable from
the Web UI. Project-scoped agents continue to work and also appear in the
same panel so the operator sees every connected agent in one view —
"global" is just one possible scope value.

## Locked-in design decisions

These aren't negotiable without a fresh round of discussion.

### Agent lifecycle

- **`dx agent connect`** is the primary verb. Registers with the server
  via the existing WS handshake and either:
  - **starts the work loop immediately** (default), or
  - **stays idle** with `--idle`, holding only the WS connection so the
    operator can pause/resume/drain/dispatch from the UI.
- **`--global` is a flag on `connect` itself, not on the parent.** Order:
  `dx agent connect --global` (NOT `dx agent --global connect`). Reasoning:
  global is a connection-manager concern, not part of the agent's
  identity. *Phase 1 currently has it as a parent persistent flag — small
  fix scheduled in phase 2 (~5 lines).*
- **Project-scoped agents are scope-immutable.** Re-register if you want
  to change scope. Less footgun surface.
- **Global agents can be pinned to a project** via assign/unassign. The
  agent stays visible in the global pool while pinned — pinning doesn't
  convert the scope, just expresses preference.

### Behavior of an unassigned global agent

- **Idle by default**, until either pinned to a project (claims that
  project's queue) or the operator promotes a todo to high priority
  somewhere it can see (queue browsing — see "cross-project priority"
  below).

### Cross-project work for assigned global agents

- Once pinned to a project, the agent claims from that project's queue
  exactly like a project-scoped agent.

### Cross-project priority (deferred — phase 3)

- Project-priority × todo-priority composite score. Global browsers
  pick the highest composite work item across every project they have
  access to.
- **Implementation has a real complexity cost**: needs a server-wide
  cached priority list (a "global queue view") because re-scanning every
  project's queue on every claim is too expensive. Defer until we have a
  clear cost-model for the cache invalidation strategy.

### Operator → agent dispatch (phase 3)

- **No push semantics.** Operator does not send "agent X work issue Y".
- Instead, **"mark as priority"** boosts the todo's queue position. The
  agent's normal claim path picks it up at the next iteration. Cleaner —
  one mechanism (queue) rather than two (queue + push).

### Workspace relocation (deferred — significant)

- All workspaces move to `~/.zdx/workspaces/<project>/...` —
  global-pool agents and project-scoped agents alike use this layout.
- Today: `./.zdx/agent/slots/<alias>/` lives inside the operator's
  project root. That's the source of the `git worktree` + bind-mount
  complexity in `mcp_container.go`.
- New layout has the agent's working tree decoupled from the
  operator's project tree by default — fixes the operator-vs-agent
  collision class at the directory level, not just per-slot.
- Touches: agent provisioning, `bin/ship`'s view of the workspace,
  the dx-config lookup chain, the dx-agent MCP server's `--mcp-root`.
- Not a phase 2 blocker — phase 2 ships against the current layout.
  Schedule a focused pass for it.

### Authentication

- `dx agent connect --global` should use a server-admin token, not a
  project API key. Today's WS auth is `?api_key=…` (project token) which
  *technically* works for the WS upgrade but is wrong semantically.
- Phase 3 wires the admin-token path properly. Until then, document the
  workaround (operator with admin-tier API key uses it for global
  registration).

### Heartbeat / liveness

- `last_heartbeat` is the source of truth. The WS connection itself is
  the liveness signal in real time; the column persists the
  last-known-alive timestamp so the UI can show "online 4m ago" for
  agents that disconnected without a clean shutdown.
- **Reap** (`dx agent reap`) deletes rows older than a threshold. Today
  it sweeps per-project. For globals: separate admin-triggered reap
  rather than a per-project sweep that might accidentally reap a global
  agent that was just slow to heartbeat.

### Web UI is the primary control surface

- Everything except the initial `connect` happens via WebUI: pause,
  resume, drain, assign, unassign, mark-as-priority.
- The CLI commands for these actions remain (don't remove them) but
  aren't expected to be the operator's daily-driver path.

## Phase 1 (SHIPPED — commit `21bc378f`)

- ✅ Schema migration `148_agents_global_pool`: `project_id` nullable,
  `idle BOOLEAN NOT NULL DEFAULT false`, partial index on global pool
- ✅ WS handshake (`/api/agents/connect`): `project_slug`, `idle` fields
- ✅ `agentdaemon.Daemon`, `agentconn.Conn`, `ProviderOpts` carry the new
  fields
- ✅ `dx agent connect [--idle]` verb (currently `dx agent --global
  connect` — see "small fix" below)
- ✅ `GET /api/agents` server-wide list endpoint (joins
  `zdx_projects.slug + name`)
- ✅ `AgentItem` extended: `ProjectID *int32`, `ProjectSlug`,
  `ProjectName`, `Idle`

## Phase 2 — UI panel + pin/unpin

Goal: operator can see every connected agent in one place and pin/unpin
global agents to projects from the UI.

- [ ] **Small fix**: move `--global` flag from parent persistent to
  `connect`-local. ~5 lines, mechanical. Drop the parent-level
  declaration; add to `agentConnectCmd`'s flag set.
- [ ] **`POST /api/agents/{id}/assign`** body `{project_slug}`:
  - 400 if agent's `project_id` is non-NULL (project-scoped agents are
    scope-immutable; reject the rebind path explicitly)
  - else: set `project_id` to the named project's id
- [ ] **`DELETE /api/agents/{id}/assign`**:
  - 400 if agent was *originally* registered project-scoped (we only
    permit unpinning agents registered as global)
  - else: clear `project_id` back to NULL
- [ ] Track "originally global" — needs an `original_scope` flag or a
  separate column so the assign endpoint can distinguish "scoped agent
  registered with a project" from "global agent currently pinned". One
  bit, set on first registration.
- [ ] **UI `/agents` route** + nav item:
  - Table: alias / scope / status / host / since / actions
  - Scope renders as project name (link to `/project/:slug/agents/:id`)
    or "global" for `project_id IS NULL`
  - Status from registry presence (live WS) + `idle` column +
    `status` column (paused/draining)
  - Action buttons: pause, resume, drain (existing endpoints), assign,
    unassign
  - Auto-refresh: existing audit channel WS or polling every 5s
- [ ] **UI nav placement**: top-level `/agents`, parity with
  `/projects`. New entry in the main nav component.

## Phase 3 — push behavior (becomes "mark priority"), cross-project queue, admin auth

Goal: global agents can pick up cross-project work without manual pinning;
operator escalates urgent work via "mark as priority"; auth story is
proper.

- [ ] **"Mark as priority"**: per-todo flag (or a priority-bump column)
  that overrides the normal queue ordering. Surfaces the todo at the top
  of the next claim regardless of its kind/priority. UI verb on the todo
  row + per-issue page.
- [ ] **Cross-project queue browsing for unpinned global agents**:
  - Schema: `zdx_projects.priority INT NOT NULL DEFAULT 5`
  - Server: cached cross-project priority list (`zdx_solo_global_view`?
    materialized view? trigger-rebuilt cache?). Decision deferred —
    needs a costing pass on cache-invalidation patterns.
  - Claim path: global agents (no `project_id`, not pinned) call a new
    server endpoint `POST /api/dx/solo/claim-any` that scans the cross-
    project view.
- [ ] **Admin token auth path** for `dx agent connect --global`:
  - Today's WS auth is project-tier `?api_key=…`
  - Add server-admin token recognition; reject project tokens for
    global registration (or accept both transitionally)
- [ ] **Push-work commands**: explicitly NOT BUILDING. The promotion
  mechanism replaces the dispatch pattern.
- [ ] **Dedicated reap for globals**: admin-triggered, separate from
  per-project sweeps.

## Deferred (file as tracker issues)

These aren't sequenced into a phase yet — file separately so phase work
doesn't drag them in.

- **Workspace relocation** to `~/.zdx/workspaces/<project>/...`. Big
  refactor across agent provisioning, ship hooks, dx config lookup,
  dx-agent `--mcp-root`. File as tracker issue with this section as the
  design context.
- **Project priority schema + cached cross-project queue view** —
  prerequisite for phase 3's cross-project claim. Splits naturally:
  schema first (cheap), cache strategy second (deferred until queue
  size + access pattern data).
- **"Mark as priority" UI verb** — phase 3 element but useful even
  before phase 3 lands (boosts a todo for project-scoped agents too).
- **Move `--global` from parent persistent to `connect`-local** — small
  enough to pick up at the start of phase 2 without filing a separate
  issue.

## Existing tracker issues (filed earlier in this session)

These are orthogonal streams that touch the same surfaces but aren't
blockers for phase 2:

- IS-1048 — Per-issue agent branch lifecycle (Stream 1)
- IS-1049 — Structured agent role contracts (Stream 2)
- IS-1050 — Review-as-agent pipeline (Stream 3)
- IS-1051 — Decomposition spikes (Stream 4)
- IS-1052 — Churn hints primitive (Stream 5a)
- IS-1053 — Reviewer effort highlights (Stream 5b)
- IS-1056 — NEEDS_CHANGES rework as churn ref (Stream 5c)
- IS-1057 — File-thrash observer (Stream 5d)
- IS-1058 — Live agent stats + PM/Tech hint surfaces (Stream 5e)

The global agent pool design doesn't conflict with any of these — both
build out from the same foundation.

## Pickup checklist for the next session

1. Confirm phase 1 (`21bc378f`) deployed + healthy (`./bin/dx agent
   connect --help` should show the `--idle` flag).
2. Apply the small `--global` flag relocation fix at the top of phase 2.
3. Build the assign/unassign endpoints + the `original_scope` bit.
4. Build the UI `/agents` route + nav item.
5. Wire the pause/resume/drain action buttons (endpoints exist).

After phase 2 lands, the natural next moves are: workspace relocation
(file tracker), then phase 3 mark-as-priority + cross-project queue.
