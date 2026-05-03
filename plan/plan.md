# zdx Product Restructuring Plan

## Vision

zdx: Principled SDLC for everyone, augmented by agents.

zdx is an ad-hoc SDLC platform that gives any developer — solo or on a team —
product management superpowers without requiring a PM background. It provides a
structured workflow (goals -> features -> specs -> issues -> tasks) that can be
adopted incrementally: start with `dx init` and an issue, grow into quantified
goals and maturity tracking as the project demands it. The CLI is the primary
interface for both humans and LLM agents. Agents participate as first-class
collaborators through a safe execution harness (docker + worktrees) that lets
them claim work, implement changes, and surface decisions for human review —
without risking the host system. The web UI provides visibility and owner-level
controls. Self-hosted, no vendor lock-in.

## New Goals

| # | Goal | Outcome | Metric |
|---|------|---------|--------|
| 1 | Agents ship real work safely | Agents claim todos, work in isolated environments, deliver tested branches | agent_pr_merge_rate (percent) |
| 2 | Humans stay in control | Owners triage, review, unblock with minimal effort. System surfaces decisions, not busywork | owner_decision_latency (minutes) |
| 3 | Anyone can be a PM | Developers adopt structured SDLC incrementally — the CLI teaches the workflow | onboarding_time_to_first_value (minutes) |
| 4 | Projects improve over time | Doctor + maturity vines nudge projects toward health without gating progress | projects_at_maturity_rung_3 (percent) |
| 5 | Platform runs itself | Deploy, observe, administer. Self-hosted operators need zero vendor involvement | api_uptime (percent) |

## Feature Mapping

Reparent existing features to new goals, consolidate where obvious.

### Goal 1: Agents ship real work safely
- dx-todo (keep, reparent from G-9)
  - dx-todo-queue (reparent)
  - dx-todo-persistence (reparent)
  - dx-todo-agent (reparent)
- (NEW) agent-harness — safe execution environment: docker containers, worktree isolation, resource limits, sandboxed shell
- Consolidate 6 test/coverage features into 1-2 (verification infrastructure)

### Goal 2: Humans stay in control
- blocker-questions + cli + ui children (reparent from G-9)
- dx-todo-tasks (reparent — it's the human lifecycle: triage, review, done/undone)

### Goal 3: Anyone can be a PM
- (NEW) sdlc-workflow — the goal/feature/spec/issue/task hierarchy, CLI commands, incremental adoption
- (NEW) onboarding — dx init, dx doctor --fix, bootstrap guidance

### Goal 4: Projects improve over time
- project maturity guidance (reparent from G-8)
- (NEW) doctor-maturity-vines — classification-specific health checks and auto-fix

### Goal 5: Platform runs itself
- admin-llm-config (reparent from G-12)
- admin-project-management (reparent from G-12)
- stable deployments (reparent from G-12)

## Deprecations

### MCP tool server (dx mcp)
The current MCP server mirrors the CLI with a separate code path that drifts.
Agents with shell access can call `dx` directly. Plan:
1. Stop adding features/specs to MCP tools
2. Migrate agent loop to use CLI calls instead of MCP tool calls
3. Remove MCP server once nothing depends on it

### Old goals to archive
Once features are reparented:
- G-9 Collaborative Dev -> archived
- G-11 Correctness -> archived
- G-10 Experience -> archived
- G-7 Install & Distribution -> archived
- G-8 Planning & Maturity -> archived
- G-12 Platform Operations -> archived
- G-13 Observability -> folded into G-5 (Platform runs itself)

## Execution Order

1. ~~**Write vision**~~ — done (in this file)
2. ~~**Create new goals**~~ — done: G-14 through G-18
3. ~~**Reparent features**~~ — done: all 19 existing features reparented, categories updated
4. ~~**Archive old goals**~~ — done: G-7 through G-13 archived
5. ~~**Create missing features**~~ — done: agent-harness, sdlc-workflow, onboarding, doctor-maturity-vines
6. ~~**Consolidate test features**~~ — done: 7 → 3 (dx test unified absorbed 4, kept demo recordings + feature test layers)
7. ~~**Deprecate MCP server**~~ — done: IS-465 migrated agent loop to CLI calls, removed `dx mcp`, dropped `.mcp.json` zdx registration, updated agent prompts
8. ~~**Project vision model**~~ — done: migration 106 adds title+description to zdx_projects, API endpoint, CLI `dx vision show/set`, specs on sdlc-workflow + onboarding
9. ~~**Set zdx vision on production**~~ — done
10. ~~**Wire vision into doctor/agent context**~~ — done: has_vision doctor check in identity rung, agent sessions get vision in prompt

---

# Unified Event Stream + Agent-Processed Comments

Replaces `CommentsAndRevisions` everywhere with one event-stream component. Folds revisions, status changes, agent actions, etc. into a single typed event log per target. Every user message must be processed by an agent (addressed / already-addressed / for-someone-else) — no silent ignores.

## Principles

- One linear event stream per `(target_type, target_id)`. Chronological. Many event types.
- Threads are flat — one layer. A reply auto-promotes the parent message to a thread head. Replies inside a thread stay in that thread.
- Original message stays in the main stream after a thread spawns from it. Clicking it activates the thread filter (right-side panel lists threads; pick one to filter the main stream).
- Every user message carries an `agent_process_result`. If newest user message is older than `stream.last_evaluated_at`, the stream is current. Otherwise an agent owes a verdict.
- Each event type defines: a React renderer, a `summary_json` serializer (cheap agent context), a `detail_json` serializer (inspect). Renderers are a registry keyed by `event_type`.

## Data model

New tables (replace `zdx_comments` + revision tables on migration):

```
zdx_event_streams
  project_id, target_type, target_id     -- composite key
  last_evaluated_at timestamptz nullable
  last_evaluated_by text                  -- agent id

zdx_events
  id bigserial pk
  project_id int
  target_type text, target_id text        -- denormalized for stream queries
  thread_id bigint nullable               -- null = root in main stream
  event_type text                         -- comment | revision | status_change | approval | agent_action | ...
  author text, author_kind text           -- user | agent | system
  summary_json jsonb                      -- light, for agent prompts
  detail_json jsonb                       -- full payload, for inspect
  agent_process_result jsonb nullable     -- only on author_kind=user comments; see below
  created_at timestamptz

zdx_event_threads
  id bigserial pk
  project_id int
  target_type text, target_id text
  root_event_id bigint                    -- the message that spawned the thread
  title text nullable                     -- default derived from root summary
  created_at timestamptz

(Reactions table keeps shape, retargets comment_id -> event_id.)
```

`agent_process_result` shape:
```json
{
  "status": "addressed" | "already_addressed" | "for_someone_else",
  "reasoning": "...",
  "addressing_event_id": 123,    // nullable; link to the agent_action / referenced issue / etc.
  "addressed_at": "...",
  "addressed_by": "agent-id"
}
```

## Event type registry

Initial set (extensible):
- `comment` (user / agent / system)
- `revision` — body/title edited (replaces `zdx_proposal_versions` and similar)
- `status_change` — proposed→approved, open→closed, etc.
- `approval` — proposal approved as issue, includes link
- `agent_action` — agent did something (claimed todo, posted PR, ran command); `summary_json` is one-liner, `detail_json` includes full transcript ref
- `metric_update` — feature metric changed (where applicable)
- `link` — cross-reference created (issue↔proposal, plan step↔issue)
- `reaction` — emoji reaction. Hidden from UI by default; included in agent `summary_json` so LLMs can read sentiment/endorsement. Replaces the `zdx_comment_reactions` table.

Each event type declares an `audience` set ∈ {ui, agent}. Default `{ui, agent}`; `reaction` defaults to `{agent}` only. The `<EventStream />` filters by audience; agent serializers include all events regardless.

Each lives in `internal/events/types/<type>.go` with a `Renderer` interface (Summary, Detail, ReactComponentName, Audience).

## UI

Single component `<EventStream targetType targetId />` replaces every `<CommentsAndRevisions />` callsite. Behavior:

- Main pane: chronological events. Each rendered by its type-specific component.
- If any thread exists, a thread icon appears top-right. Toggles a right-side panel.
- Right panel: list of threads (title, last activity, message count, unprocessed-count badge). Click filters main pane to that thread. Click again or "show all" returns to full stream.
- Replying to any non-thread message: reply input opens inline → on submit, server creates thread (if none yet for that root) and the new reply event with `thread_id`.
- User messages with no `agent_process_result` show a small "pending review" chip; with a verdict, show the verdict (color-coded). Click chip → inspect drawer with reasoning + addressing link.

## Agent contract

Any agent loop touching a stream:
1. Read `last_evaluated_at`.
2. Fetch user-authored events newer than that with no `agent_process_result`.
3. For each: produce a verdict + reasoning, optionally take action and link via `addressing_event_id`.
4. Update `last_evaluated_at`.

Agents read events as `summary_json[]` first; expand to `detail_json` only when they choose to.

For the proposal flow specifically (the original motivator): proposal-stream agent processes user comments → can rewrite proposal body (creates a `revision` event) → links the revision as the addressing event for the comment(s) it addressed.

## Migration (one-time)

Migration NNN (next free number):

1. Create `zdx_events`, `zdx_event_threads`, `zdx_event_streams`.
2. For each row in `zdx_comments`:
   - If `parent_id IS NULL`: insert as event with `thread_id = NULL`.
   - If `parent_id IS NOT NULL`: ensure a thread row exists for the root (`root_event_id = parent's new event id`); insert child with that `thread_id`.
3. For each row in `zdx_proposal_versions` (and any sibling version tables): insert as `revision` event on the corresponding target.
4. `agent_process_result` is `NULL` for all backfilled events (treat existing thread as pre-revamp; agent loop won't retroactively process).
5. Drop `zdx_comment_reactions` (reactions become events going forward; existing reactions are not migrated).
6. Drop `zdx_comments`, `zdx_comment_reads`, version tables in a follow-up migration after callsites updated.

## Execution order

1. Schema migration + sqlc queries (events, threads, streams).
2. Event type registry skeleton (Go interface, comment + revision + status_change implementations).
3. Server endpoints: `GET /events`, `POST /events` (comment), `POST /events/:id/reply`, `POST /streams/:target/process` (agent), `PATCH /threads/:id` (title).
4. Migration NNN with backfill + integration test verifying stream contents preserved across upgrade.
5. UI `<EventStream />` component + `<ThreadPanel />`. Replace one callsite (Proposal) end-to-end. Iterate.
6. Roll out to remaining callsites (task, journal, review, question, spec, feature, pattern, issue, focus, plan, goal). One PR per surface to keep diffs reviewable.
7. Drop legacy tables.
8. Wire proposal-stream agent loop (closes the original "comments don't refine the proposal" gap).
9. Generalize the agent loop to all streams (any target with unprocessed user messages).

## IS-840 Prod Cleanup — Pending User Review

Confirmed fixture projects deleted 2026-05-02 (see IS-840 comment C-481):
- `demo-add-help-guidance` (id=4) — PATTERN+EMPTY
- `demo-doctor-vine-classification` (id=5) — PATTERN+EMPTY

Borderline projects with single EMPTY flag — need owner decision before deletion:
- `tasky` (id=2) — 0 issues/features, 6 auto-generated doctor todos
- `visionbridge` (id=3) — completely empty
- `iodesystems` (id=7) — 0 issues/features, 11 auto-generated todos (created 2026-05-02)

## Resolved

- Reactions are a `reaction` event type, not a separate table. Hidden from UI by default; visible to agents via `summary_json`. Drops `zdx_comment_reactions` on migration. (2026-04-29)
- No status_change backfill. `status_change` events start being logged at migration time; pre-migration history shows no status_change events. (2026-04-29)
- Drop `zdx_comment_reads` entirely. Read-state tracking is not carried into the new model. (2026-04-29)
