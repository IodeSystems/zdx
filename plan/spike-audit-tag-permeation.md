# Spike — propagate `alias=<agent>` tag through every server-side mutation

## Problem

`dx log tail --tag alias=smoke4-0` should show the **whole flow** an
agent caused: its own loop iterations *and* every server-side mutation
the agent triggered (reviews submitted, comments added, issues closed,
todos bumped, proposals approved, etc.). Today only the loop's own
events carry the alias tag; mutations triggered by the agent's tool
calls drop it.

Concrete demo (smoke4 run, 2026-05-06):

The loop at alias `smoke4-0` produced clean tagged events for every
iteration boundary:

```
loop.started            alias=smoke4-0
crash_recovery.released_orphan
daemon.connected        alias=smoke4-0
claim.acquired          alias=smoke4-0  todo_kind=dev      todo_key=dev-TK-1700
session.start           alias=smoke4-0  issue_id=IS-982
session.end             alias=smoke4-0  status=ok
claim.released          alias=smoke4-0  cycle_detected=true
claim.acquired          alias=smoke4-0  todo_kind=read:comments
session.start           alias=smoke4-0
… (repeats per iteration: dev → read:comments → closable → …)
```

But the actual mutations the agent performed (it committed a feature,
likely posted comments, marked task done) **didn't** surface in the
filter. Server-side handlers either don't read the
`X-ZDX-Agent-Id` / `X-ZDX-Session-Id` headers from request ctx, or read
them but don't include them on broker payloads / tracelog events. So
`dx log tail --tag alias=smoke4-0` is blind to those mutations.

The cli.Client already sets the headers on every outbound request
(`attachAttributionHeaders` in `internal/cli/client.go`); they're
sitting on the request ctx unused by most handlers.

## Coverage today (handlers that DO read attribution)

```
$ grep -rn "ctxAgentIDVal\|ctxSessionIDVal" internal/server/handlers/ \
    | grep -v context.go | grep -v _test.go
6 sites in 2 files:
  handlers_comments.go   — comments add/edit
  handlers_proposals.go  — proposal create/edit
```

Everything else is dark to attribution: reviews, todo state changes,
issue close, task done, focus add/remove, plan steps, vision edits, …

## Design — standardized audit helper

Single helper that mutation handlers call once. It does three things:

1. Reads `ctxAgentIDVal` / `ctxSessionIDVal` / `UserIDFromContext` from
   ctx.
2. Merges those into the broker payload as `agent_id` /
   `session_id` / `user_id` keys.
3. Emits a tracelog event tagged with `alias=<agent_id>` so
   `dx log tail --tag alias=X` surfaces it.

```go
// internal/server/handlers/audit.go (new)

// Note records a server-side mutation event, attributing it to the
// agent + session + user from ctx. Single call site replaces:
//   - copy-pasted ctxAgentIDVal lookups
//   - h.Broker.PublishX without attribution
//   - missing tracelog emit on mutations
//
// Call once per mutation, after the DB write succeeded:
//
//   audit.Note(ctx, h, "task.reviewed",
//       "task_id", id, "verdict", verdict, "review_id", rev.ID,
//       "broker_target", brokerTaskTarget(p.Slug, id))
//
// Pass broker_target as a special key when the existing broker
// channel matters; otherwise it just emits to tracelog + a generic
// audit channel.
func Note(ctx context.Context, h *Handler, eventType string, kv ...any)
```

## Persistence (audit-critical rows)

Some rows are queried directly by operators ("which agent reviewed this
task?"). For those, persist `agent_id` + `session_id` columns in
addition to broker/tracelog. Candidates:

- `zdx_task_reviews`        — reviewer attribution (most useful)
- `zdx_issue_status_changes` — issue closes, reopens
- `zdx_todos.last_resolved_by` — already exists? Check.
- `zdx_blocker_questions` answers — who answered

Each is a small migration + one `audit.Note` call site updated to also
pass through to the DB query.

## Threading scope (handler sites to update)

Run this query to find them all:
```bash
grep -rn "h\.Broker\.Publish\|UpsertTodo\|UpsertTaskReview\|MarkTask\|UpdateIssue\|CloseIssue\|InsertComment" internal/server/handlers/
```

Estimated 15–25 handler sites. Group by domain:

- **task lifecycle** (handlers_tasks.go, handlers_todos.go) — mark
  done, mark reviewed, set priority, claim, release
- **issue lifecycle** (handlers_issues.go) — close, reopen, edit
  context, ready
- **proposals** (handlers_proposals.go) — already has 2 sites,
  finish the rest
- **focus** — add/remove blockers
- **comments** — already covered
- **plans** — step add/update
- **blocker questions** — answer

## Implementation phases

**Phase 1** — helper lands. `audit.Note` exists, has unit tests.
No handlers use it yet. ~1 hour.

**Phase 2** — convert one domain end-to-end (recommend
**handlers_tasks.go's review handler** since that's the user's
trigger). Validates the helper's shape against a real mutation +
a broker event + a persistent column. Migrate
`zdx_task_reviews.agent_id` + `session_id` columns at the same
time. ~1 hour.

**Phase 3** — fan out to remaining handlers. Each is a 5-10 line
diff once the helper is set. ~1-1.5 hours.

**Phase 4** — verification: run `dx agent loop` against a real
issue, confirm `dx log tail --tag alias=X` shows everything the
agent caused, including the review verdict landing as
`task.reviewed alias=X verdict=approve`.

Total sized: 3-4 hours focused, single sitting recommended so the
helper's shape stays consistent across the fan-out.

## Concrete demo target

After this lands, the smoke4-style chain should look like this in
`dx log tail --tag alias=smoke4-0`:

```
loop.started
claim.acquired      todo_kind=dev
session.start
todo.bumped         alias=smoke4-0  ... (NEW)
comment.added       alias=smoke4-0  ... (NEW; today: dropped)
issue.context_edit  alias=smoke4-0  ... (NEW)
task.done           alias=smoke4-0  ... (NEW)
session.end
claim.released      cycle_detected=true
claim.acquired      todo_kind=read:comments
session.start
comment.added       alias=smoke4-0  ... (NEW)
session.end
claim.released
claim.acquired      todo_kind=closable
session.start
issue.closed        alias=smoke4-0  ... (NEW)
session.end
```

Plus per-row queries:
```sql
SELECT * FROM zdx_task_reviews WHERE agent_id = 'smoke4-0' ORDER BY created_at DESC;
```

## Out of scope (file separately)

- **Cross-project audit panel** in `/admin/activity` — natural
  next step after this lands, but UI work, not the core fix.
- **Trace correlation across slot↔host boundaries** — the in-slot
  `dx-agent --mcp-stdio` could log structured events too. Today
  it doesn't. Nice-to-have, not blocking the audit story.

## Why filing not doing tonight

- 15-25 handler sites; rushed threading produces silently-broken
  audit (worse than no audit because operators trust the trail).
- Shared helper shape benefits from a clean head — getting it
  wrong forces a re-fan-out.
- Live agent run currently burning tokens at the time of writing
  (smoke4 loop iterating); higher priority is wrapping up the
  live verification of GAPD's executor refactor.
