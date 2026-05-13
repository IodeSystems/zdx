-- name: ListTodos :many
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
       claim_base_sha, claim_base_branch
FROM zdx_todos WHERE project_id = $1 ORDER BY priority, created_at;

-- name: ListTodosFiltered :many
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
       claim_base_sha, claim_base_branch
FROM zdx_todos
WHERE project_id = @project_id
  AND (sqlc.narg('blocked')::boolean IS NULL OR blocked = sqlc.narg('blocked')::boolean)
  AND (sqlc.narg('target_type')::text IS NULL OR target_type = sqlc.narg('target_type')::text)
  AND (sqlc.narg('issue_ref')::text IS NULL OR issue_ref = sqlc.narg('issue_ref')::text)
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
ORDER BY priority, created_at;

-- name: DeleteTodosForProject :exec
DELETE FROM zdx_todos WHERE project_id = $1;

-- name: CreateTodo :one
INSERT INTO zdx_todos (project_id, text, title, description, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked, blocked_reason)
VALUES (@project_id, @text, @title, @description, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked, @blocked_reason)
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
          claim_base_sha, claim_base_branch;

-- name: UpsertTodo :one
-- Upsert a todo item, preserving existing claim state (claimed_by, claimed_at, lease_expires_at).
-- Tracks resolve→open churn: reopen_count increments each time a resolved key is re-emitted.
-- Auto-blocks at 3+ reopens so agents don't churn indefinitely on an untriageable item.
INSERT INTO zdx_todos (project_id, text, title, description, key, persona, priority, status,
                       target_type, target_id, kind, issue_ref, blocked, blocked_reason)
VALUES (@project_id, @text, @title, @description, @key, @persona, @priority, @status,
        @target_type, @target_id, @kind, @issue_ref, @blocked, @blocked_reason)
ON CONFLICT (project_id, key) DO UPDATE SET
  text = EXCLUDED.text,
  title = EXCLUDED.title,
  description = EXCLUDED.description,
  persona = EXCLUDED.persona,
  -- priority: take the lower (more urgent) value so operator-pushed pri stays
  -- bumped across re-evaluate. The natural priority computed for each kind is
  -- a ceiling — once an operator escalates, only an even-higher escalation
  -- (smaller integer) replaces it. To restore natural priority, write the
  -- desired value via PUT /api/dx/projects/{slug}/todos/{key}/priority.
  priority = LEAST(zdx_todos.priority, EXCLUDED.priority),
  status = CASE WHEN zdx_todos.status = 'resolved' THEN 'open' ELSE zdx_todos.status END,
  target_type = EXCLUDED.target_type,
  target_id = EXCLUDED.target_id,
  kind = EXCLUDED.kind,
  issue_ref = EXCLUDED.issue_ref,
  -- blocked: churn-guard (reopen_count >= 3) sticks; cycle-detection blocks
  -- naturally clear when the fresh candidate is not blocked. EXCLUDED.blocked
  -- otherwise wins so re-evaluation reflects current state.
  blocked = CASE
    WHEN zdx_todos.status = 'resolved' AND zdx_todos.reopen_count + 1 >= 3 THEN true
    WHEN zdx_todos.reopen_count >= 3 THEN true
    ELSE EXCLUDED.blocked
  END,
  blocked_reason = CASE
    WHEN zdx_todos.status = 'resolved' AND zdx_todos.reopen_count + 1 >= 3 THEN 'Reopened 3+ times: manually review and unblock when ready'
    WHEN zdx_todos.reopen_count >= 3 THEN 'Reopened 3+ times: manually review and unblock when ready'
    ELSE EXCLUDED.blocked_reason
  END,
  reopen_count = CASE
    WHEN zdx_todos.status = 'resolved' THEN zdx_todos.reopen_count + 1
    ELSE zdx_todos.reopen_count
  END
  -- claimed_by, claimed_at, lease_expires_at, cycle_count, reference_issue_id intentionally NOT updated
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
          claim_base_sha, claim_base_branch;

-- name: ResolveTodo :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND key = $2;

-- name: GetTodoByKey :one
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
       claim_base_sha, claim_base_branch
FROM zdx_todos WHERE project_id = $1 AND key = $2;

-- name: GetTodoByID :one
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
       claim_base_sha, claim_base_branch
FROM zdx_todos WHERE id = $1;

-- name: SetTodoPriority :exec
-- Operator-driven push: set the todo's priority directly. UpsertTodo's
-- LEAST() clause preserves the value across re-evaluate, so a push sticks
-- until another operator write or until natural priority drops below it.
UPDATE zdx_todos SET priority = @priority
WHERE project_id = @project_id AND key = @key;

-- name: ResolveTodosNotInKeys :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW()
WHERE project_id = $1 AND status = 'open' AND key != ALL(@keys::text[]);

-- name: ClaimNextTodo :one
-- Atomically claim the highest-priority unclaimed open todo for an agent.
-- Skips locked rows (concurrent agents get different items).
-- target_branch is resolved from the referenced issue (default 'dev').
-- claim_persona records the role the agent claimed AS (IS-1096); separate
-- from zdx_todos.persona which advertises the persona the todo is FOR.
WITH claimed AS (
  UPDATE zdx_todos SET
    claimed_by        = @agent_id,
    claimed_at        = NOW(),
    lease_expires_at  = NOW() + (@lease_minutes::int || ' minutes')::interval,
    claim_base_sha    = @claim_base_sha,
    claim_base_branch = @claim_base_branch,
    claim_persona     = @claim_persona
  WHERE id = (
    SELECT t.id FROM zdx_todos t
    WHERE t.project_id = @project_id
      AND t.status = 'open'
      AND t.blocked = false
      AND (t.claimed_by = '' OR t.lease_expires_at < NOW())
    ORDER BY t.priority, t.created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
  )
  RETURNING id, project_id, text, title, description, key, persona, priority, status,
            target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
            claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
            claim_base_sha, claim_base_branch, claim_persona
)
SELECT c.id, c.project_id, c.text, c.title, c.description, c.key, c.persona, c.priority, c.status,
       c.target_type, c.target_id, c.kind, c.issue_ref, c.blocked, c.blocked_reason, c.cycle_count, c.reference_issue_id,
       c.claimed_by, c.claimed_at, c.lease_expires_at, c.created_at, c.resolved_at, c.reopen_count,
       c.claim_base_sha, c.claim_base_branch, c.claim_persona,
       COALESCE(i.target_branch, 'dev') AS target_branch
FROM claimed c
LEFT JOIN zdx_issues i ON i.id = c.issue_ref;

-- name: ListIssueCompositionDescendants :many
-- Walk composition edges downward from a seed issue, returning the seed
-- itself plus every transitively-composed child. Used by the scoped
-- agent claim path to restrict candidate todos to a tracker's subtree.
-- Composition edge layout (matches queries/issue_blocks.sql): the row
-- (issue_id=parent, blocked_by_id=child, kind='composition') means
-- "child belongs to parent". Walking parent→children means joining
-- descendants.id = blocks.issue_id and emitting blocks.blocked_by_id.
WITH RECURSIVE descendants(id) AS (
  SELECT @issue_id::text
  UNION
  SELECT b.blocked_by_id
  FROM zdx_issue_blocks b
  JOIN descendants d ON d.id = b.issue_id
  WHERE b.kind = 'composition'
)
SELECT id FROM descendants;

-- name: ClaimNextTodoInScope :one
-- Scoped variant of ClaimNextTodo: only considers todos whose issue_ref
-- is in the provided scope set (typically the scope issue + its
-- composition descendants from ListIssueCompositionDescendants).
-- Preserves FOR UPDATE SKIP LOCKED and priority ordering so concurrent
-- scoped agents don't collide.
WITH claimed AS (
  UPDATE zdx_todos SET
    claimed_by        = @agent_id,
    claimed_at        = NOW(),
    lease_expires_at  = NOW() + (@lease_minutes::int || ' minutes')::interval,
    claim_base_sha    = @claim_base_sha,
    claim_base_branch = @claim_base_branch,
    claim_persona     = @claim_persona
  WHERE id = (
    SELECT t.id FROM zdx_todos t
    WHERE t.project_id = @project_id
      AND t.status = 'open'
      AND t.blocked = false
      AND (t.claimed_by = '' OR t.lease_expires_at < NOW())
      AND t.issue_ref = ANY(@scope_issue_ids::text[])
    ORDER BY t.priority, t.created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
  )
  RETURNING id, project_id, text, title, description, key, persona, priority, status,
            target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
            claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
            claim_base_sha, claim_base_branch, claim_persona
)
SELECT c.id, c.project_id, c.text, c.title, c.description, c.key, c.persona, c.priority, c.status,
       c.target_type, c.target_id, c.kind, c.issue_ref, c.blocked, c.blocked_reason, c.cycle_count, c.reference_issue_id,
       c.claimed_by, c.claimed_at, c.lease_expires_at, c.created_at, c.resolved_at, c.reopen_count,
       c.claim_base_sha, c.claim_base_branch, c.claim_persona,
       COALESCE(i.target_branch, 'dev') AS target_branch
FROM claimed c
LEFT JOIN zdx_issues i ON i.id = c.issue_ref;

-- name: ListOpenIssuesInSet :many
-- Return the subset of @ids that are currently open issues. Used by
-- the scoped-claim stall path to distinguish "all descendants closed"
-- (tracker-closable) from "some open but all blocked/reserved".
SELECT id FROM zdx_issues
WHERE id = ANY(@ids::text[])
  AND status = 'open';

-- name: ClaimNextTodoAny :one
-- Cross-project atomic claim for unpinned global agents. Picks the
-- single best todo across every project, ordered by project.priority
-- (lower=earlier, default 5), then todo.priority, then created_at.
-- The composite ordering means a low-priority project can still
-- starve out a high-priority project's stale low-priority work, and
-- operator priority bumps (LEAST-preserved on UpsertTodo) cut through
-- the same way they do per-project. Same FOR UPDATE SKIP LOCKED
-- semantics as ClaimNextTodo so concurrent global agents don't
-- collide. Returns project_slug so the caller can route follow-up
-- API calls to the right project namespace.
WITH claimed AS (
  UPDATE zdx_todos SET
    claimed_by        = @agent_id,
    claimed_at        = NOW(),
    lease_expires_at  = NOW() + (@lease_minutes::int || ' minutes')::interval,
    claim_base_sha    = @claim_base_sha,
    claim_base_branch = @claim_base_branch,
    claim_persona     = @claim_persona
  WHERE id = (
    SELECT t.id FROM zdx_todos t
    JOIN zdx_projects p ON p.id = t.project_id
    WHERE t.status = 'open'
      AND t.blocked = false
      AND (t.claimed_by = '' OR t.lease_expires_at < NOW())
    ORDER BY p.priority, t.priority, t.created_at
    LIMIT 1
    FOR UPDATE OF t SKIP LOCKED
  )
  RETURNING id, project_id, text, title, description, key, persona, priority, status,
            target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
            claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
            claim_base_sha, claim_base_branch, claim_persona
)
SELECT c.id, c.project_id, c.text, c.title, c.description, c.key, c.persona, c.priority, c.status,
       c.target_type, c.target_id, c.kind, c.issue_ref, c.blocked, c.blocked_reason, c.cycle_count, c.reference_issue_id,
       c.claimed_by, c.claimed_at, c.lease_expires_at, c.created_at, c.resolved_at, c.reopen_count,
       c.claim_base_sha, c.claim_base_branch, c.claim_persona,
       COALESCE(i.target_branch, 'dev') AS target_branch,
       p.slug AS project_slug
FROM claimed c
LEFT JOIN zdx_issues i ON i.id = c.issue_ref
JOIN zdx_projects p ON p.id = c.project_id;

-- name: ReleaseTodo :exec
-- Release a claimed todo (agent finished or abandoned).
UPDATE zdx_todos SET
  claimed_by        = '',
  claimed_at        = NULL,
  lease_expires_at  = NULL,
  claim_base_sha    = '',
  claim_base_branch = ''
WHERE id = $1 AND claimed_by = $2;

-- name: ReleaseTodoAdmin :exec
-- Admin release: clear the claim unconditionally (no agent_id check).
UPDATE zdx_todos SET
  claimed_by        = '',
  claimed_at        = NULL,
  lease_expires_at  = NULL,
  claim_base_sha    = '',
  claim_base_branch = ''
WHERE id = $1;

-- name: RenewTodoLease :exec
-- Extend the lease on a claimed todo (heartbeat).
UPDATE zdx_todos SET
  lease_expires_at = NOW() + (@lease_minutes::int || ' minutes')::interval
WHERE id = @id AND claimed_by = @agent_id;

-- name: ResolveTodoByID :exec
UPDATE zdx_todos SET status = 'resolved', resolved_at = NOW(),
  claimed_by = '', claimed_at = NULL, lease_expires_at = NULL,
  claim_base_sha = '', claim_base_branch = ''
WHERE id = $1;

-- name: BlockTodoByKey :one
-- Block a todo by its key (cycle detection). Increments cycle_count each time.
-- Returns the updated row so callers can check whether to auto-file an issue.
UPDATE zdx_todos SET blocked = true, blocked_reason = @reason, cycle_count = cycle_count + 1
WHERE project_id = @project_id AND key = @key
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
          claim_base_sha, claim_base_branch;

-- name: SetTodoReferenceIssue :exec
-- Store the auto-filed issue ID on a blocked todo so the UI can link to it.
UPDATE zdx_todos SET reference_issue_id = @reference_issue_id
WHERE project_id = @project_id AND key = @key;

-- name: UnblockTodosByReferenceIssue :exec
-- When the referenced issue is closed/fixed, automatically unblock the todo.
UPDATE zdx_todos SET blocked = false, blocked_reason = '', reference_issue_id = ''
WHERE project_id = @project_id AND reference_issue_id = @reference_issue_id AND blocked = true AND status = 'open';

-- name: UnblockAllTodos :exec
-- Clear blocked flag and reopen_count on all blocked todos for a project.
-- reopen_count is reset because the upsert re-block guard (queries/todos.sql:56-65)
-- fires on reopen_count >= 3 and would otherwise immediately re-block every row on
-- the next claim's upsert. cycle_count is preserved so the auto-file threshold is
-- not reset by a manual admin unblock.
UPDATE zdx_todos SET blocked = false, blocked_reason = '', reference_issue_id = '', reopen_count = 0
WHERE project_id = @project_id AND blocked = true AND status = 'open';

-- name: ReclaimExpiredTodos :many
-- Clear claims on todos whose leases have expired. Returns affected rows for reservation release.
UPDATE zdx_todos SET
  claimed_by        = '',
  claimed_at        = NULL,
  lease_expires_at  = NULL,
  claim_base_sha    = '',
  claim_base_branch = ''
WHERE project_id = $1
  AND claimed_by != ''
  AND lease_expires_at < NOW()
RETURNING id, project_id, text, title, description, key, persona, priority, status,
          target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
          claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
          claim_base_sha, claim_base_branch;

-- name: ListActiveTodoClaims :many
-- Return all todos that are currently claimed and whose lease has not expired.
SELECT id, project_id, text, title, description, key, persona, priority, status,
       target_type, target_id, kind, issue_ref, blocked, blocked_reason, cycle_count, reference_issue_id,
       claimed_by, claimed_at, lease_expires_at, created_at, resolved_at, reopen_count,
       claim_base_sha, claim_base_branch
FROM zdx_todos
WHERE project_id = $1
  AND claimed_by != ''
  AND lease_expires_at > NOW()
ORDER BY claimed_at DESC;

-- name: CountUnclaimedTodos :one
-- Count open, unblocked todos that are not currently claimed (or whose lease has expired).
-- Used by the /api/health queue subsystem probe to surface backlog depth.
SELECT COUNT(*) FROM zdx_todos
WHERE status = 'open'
  AND blocked = false
  AND (claimed_by = '' OR lease_expires_at < NOW());

-- name: GetTodoQueueHealth :one
-- Aggregate open todo queue health: total open, blocked count, unblocked count,
-- and the single most common blocked_reason (null if none).
SELECT
  COUNT(*) FILTER (WHERE t.status = 'open')                        AS total_open,
  COUNT(*) FILTER (WHERE t.status = 'open' AND t.blocked = true)   AS blocked_count,
  COUNT(*) FILTER (WHERE t.status = 'open' AND t.blocked = false)  AS unblocked_count,
  (SELECT sub.blocked_reason
   FROM zdx_todos sub
   WHERE sub.project_id = $1 AND sub.status = 'open' AND sub.blocked = true AND sub.blocked_reason != ''
   GROUP BY sub.blocked_reason ORDER BY COUNT(*) DESC LIMIT 1)     AS dominant_blocked_reason
FROM zdx_todos t
WHERE t.project_id = $1;

-- name: GetState :one
SELECT value FROM zdx_state WHERE project_id = $1 AND key = $2;

-- name: SetState :exec
INSERT INTO zdx_state (project_id, key, value, updated_at)
VALUES (@project_id, @key, @value, NOW())
ON CONFLICT (project_id, key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW();
