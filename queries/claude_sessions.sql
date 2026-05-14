-- name: CreateClaudeSession :one
INSERT INTO zdx_claude_sessions (project_id, issue_id, session_id, title, alias, todo_id, persona)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at, todo_id, persona;

-- name: GetClaudeSession :one
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id, s.persona,
       t.text AS todo_text, t.title AS todo_title, t.description AS todo_description, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1 AND s.id = $2;

-- name: GetClaudeSessionBySessionID :one
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id, s.persona,
       t.text AS todo_text, t.title AS todo_title, t.description AS todo_description, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1 AND s.session_id = $2;

-- name: ListClaudeSessions :many
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id, s.persona,
       t.text AS todo_text, t.title AS todo_title, t.description AS todo_description, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1
ORDER BY s.updated_at DESC;



-- name: ListClaudeSessionsByIssue :many
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id, s.persona,
       t.text AS todo_text, t.title AS todo_title, t.description AS todo_description, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1 AND s.issue_id = $2
ORDER BY s.updated_at DESC;

-- name: ListClaudeSessionsByTodoID :many
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id
FROM zdx_claude_sessions s
WHERE s.project_id = $1 AND s.todo_id = $2
ORDER BY s.updated_at DESC;

-- name: CountClaudeSessionsByIssue :one
SELECT count(*) FROM zdx_claude_sessions WHERE project_id = $1 AND issue_id = $2;

-- name: UpdateClaudeSessionSummary :exec
UPDATE zdx_claude_sessions
SET header = $3, summary = $4, status = $5
WHERE project_id = $1 AND id = $2;

-- name: TouchClaudeSession :exec
UPDATE zdx_claude_sessions
SET updated_at = NOW()
WHERE id = $1;

-- name: CloseClaudeSession :exec
UPDATE zdx_claude_sessions
SET closed_at = NOW(), updated_at = NOW()
WHERE id = $1 AND closed_at IS NULL;

-- name: CloseStaleClaudeSessions :many
UPDATE zdx_claude_sessions
SET closed_at = NOW(),
    updated_at = NOW(),
    status = CASE WHEN status = '' THEN 'orphaned' ELSE status END
WHERE closed_at IS NULL
  AND updated_at <= NOW() - make_interval(mins => @stale_minutes::int)
RETURNING id, project_id, session_id;

-- name: ListStaleOpenClaudeSessions :many
SELECT id, project_id, session_id, title, alias, updated_at
FROM zdx_claude_sessions
WHERE project_id = $1
  AND closed_at IS NULL
  AND updated_at <= NOW() - make_interval(mins => @stale_minutes::int)
ORDER BY updated_at ASC;

-- name: CountStaleOpenClaudeSessions :one
SELECT count(*) FROM zdx_claude_sessions
WHERE project_id = $1
  AND closed_at IS NULL
  AND updated_at <= NOW() - make_interval(mins => @stale_minutes::int);

-- name: CreateClaudeEvent :exec
INSERT INTO zdx_claude_events (session_pk, seq, event_type, event_json, agent_id, is_sidechain, agent_type, agent_description)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);

-- name: ListClaudeEvents :many
SELECT id, session_pk, seq, event_type, event_json, created_at, agent_id, is_sidechain, agent_type, agent_description
FROM zdx_claude_events
WHERE session_pk = $1
ORDER BY seq;

-- name: ListClaudeEventsSinceSeq :many
-- Tail-shaped query for `dx agent audit --follow`: returns events with
-- seq > $2 in ascending order, capped at $3. The SSE handler polls this
-- on a 1s tick to deliver new turns to a connected operator.
SELECT id, session_pk, seq, event_type, event_json, created_at, agent_id, is_sidechain, agent_type, agent_description
FROM zdx_claude_events
WHERE session_pk = $1 AND seq > $2
ORDER BY seq
LIMIT $3;

-- name: CountClaudeEvents :one
SELECT count(*) FROM zdx_claude_events WHERE session_pk = $1;

-- name: GetMaxClaudeEventSeq :one
SELECT coalesce(max(seq), -1)::int AS max_seq FROM zdx_claude_events WHERE session_pk = $1;

-- name: ResolveClaudeSession :one
-- Resolve a session by either its session_id text OR its agent alias
-- (most-recent for that alias wins). Used by `dx agent audit <id>` so the
-- operator can pass either the UUID-shaped session_id or the agent alias
-- without having to know which they have.
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at, todo_id
FROM zdx_claude_sessions
WHERE project_id = $1
  AND (session_id = $2 OR alias = $2)
ORDER BY created_at DESC
LIMIT 1;


-- name: ListChurnSessions :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions
WHERE project_id = $1 AND status = 'churn'
  AND created_at >= $2
ORDER BY created_at DESC;

-- name: CountChurnSessions :one
SELECT count(*) FROM zdx_claude_sessions
WHERE project_id = $1 AND status = 'churn'
  AND created_at >= $2;

-- name: ListClaudeSessionsCrossProject :many
-- metaquery: off
SELECT s.id, s.project_id, p.slug AS project_slug, p.name AS project_name, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id,
       (SELECT count(*) FROM zdx_claude_events e WHERE e.session_pk = s.id) AS event_count
FROM zdx_claude_sessions s
JOIN zdx_projects p ON s.project_id = p.id
ORDER BY s.updated_at DESC
LIMIT $1 OFFSET $2;

-- name: GetClaudeSessionTokenUsage :one
SELECT
  coalesce(sum((event_json->'message'->'usage'->>'input_tokens')::bigint), 0)::bigint AS input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'output_tokens')::bigint), 0)::bigint AS output_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_read_input_tokens')::bigint), 0)::bigint AS cache_read_input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_creation_input_tokens')::bigint), 0)::bigint AS cache_creation_input_tokens
FROM zdx_claude_events
WHERE session_pk = $1
  AND event_type = 'assistant'
  AND event_json->'message'->'usage' IS NOT NULL;

-- name: CountSessionEffectiveSignals :one
-- IS-1100: returns counts of durable-mutation signals attributable to the
-- session via zdx_revisions.session_id. The close-agent-session handler emits
-- session.ineffective when every signal here is zero AND the session ran more
-- than 5 turns. Attribution: zdx_revisions.session_id is stamped by
-- recordRevision/recordStatusChange whenever a handler runs with an agent
-- session in ctx.
SELECT
  count(*) FILTER (
    WHERE field = 'status' AND new_val = 'closed'
  )::int AS closes,
  count(*) FILTER (
    WHERE target_type = 'issue' AND field IN ('issue_type', 'link_of', 'parent_id', 'duplicate_of')
  )::int AS type_parent_changes,
  count(*) FILTER (
    WHERE target_type IN ('gate_item', 'maturity_item')
  )::int AS gate_item_updates,
  count(*)::int AS total_revisions
FROM zdx_revisions
WHERE project_id = $1 AND session_id = $2;

-- name: CountSessionLongCommentsByAlias :one
-- IS-1100: count zdx_comments by the session's agent alias since the session
-- started. zdx_comments has no session_id column, so we approximate session
-- attribution by (author_alias, created_at >= session.created_at). This can
-- over-count when two sessions share an alias and overlap; v1 telemetry-only.
SELECT count(*)::int AS n
FROM zdx_comments
WHERE project_id = $1
  AND author_alias = $2
  AND created_at >= $3
  AND length(body) > 50;

-- name: GetClaudeSessionTokenUsageByAgent :many
-- metaquery: off
SELECT
  agent_id,
  agent_type,
  agent_description,
  coalesce(sum((event_json->'message'->'usage'->>'input_tokens')::bigint), 0)::bigint AS input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'output_tokens')::bigint), 0)::bigint AS output_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_read_input_tokens')::bigint), 0)::bigint AS cache_read_input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_creation_input_tokens')::bigint), 0)::bigint AS cache_creation_input_tokens,
  count(*)::bigint AS event_count
FROM zdx_claude_events
WHERE session_pk = $1
  AND event_type = 'assistant'
  AND event_json->'message'->'usage' IS NOT NULL
GROUP BY agent_id, agent_type, agent_description
ORDER BY event_count DESC;
