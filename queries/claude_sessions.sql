-- name: CreateClaudeSession :one
INSERT INTO zdx_claude_sessions (project_id, issue_id, session_id, title, alias, todo_id)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at, todo_id;

-- name: GetClaudeSession :one
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id,
       t.text AS todo_text, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1 AND s.id = $2;

-- name: GetClaudeSessionBySessionID :one
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id,
       t.text AS todo_text, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1 AND s.session_id = $2;

-- name: ListClaudeSessions :many
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id,
       t.text AS todo_text, t.target_type AS todo_target_type, t.target_id AS todo_target_id
FROM zdx_claude_sessions s
LEFT JOIN zdx_todos t ON t.id = s.todo_id
WHERE s.project_id = $1
ORDER BY s.updated_at DESC;



-- name: ListClaudeSessionsByIssue :many
SELECT s.id, s.project_id, s.issue_id, s.session_id, s.title, s.alias, s.header, s.summary, s.status, s.created_at, s.updated_at, s.closed_at, s.todo_id,
       t.text AS todo_text, t.target_type AS todo_target_type, t.target_id AS todo_target_id
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

-- name: CountClaudeEvents :one
SELECT count(*) FROM zdx_claude_events WHERE session_pk = $1;

-- name: GetMaxClaudeEventSeq :one
SELECT coalesce(max(seq), -1)::int AS max_seq FROM zdx_claude_events WHERE session_pk = $1;


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
