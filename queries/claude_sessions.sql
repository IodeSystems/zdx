-- name: CreateClaudeSession :one
INSERT INTO zdx_claude_sessions (project_id, issue_id, session_id, title)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, issue_id, session_id, title, created_at;

-- name: GetClaudeSession :one
SELECT id, project_id, issue_id, session_id, title, created_at
FROM zdx_claude_sessions WHERE project_id = $1 AND id = $2;

-- name: GetClaudeSessionBySessionID :one
SELECT id, project_id, issue_id, session_id, title, created_at
FROM zdx_claude_sessions WHERE project_id = $1 AND session_id = $2;

-- name: ListClaudeSessions :many
SELECT id, project_id, issue_id, session_id, title, created_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: CountClaudeSessions :one
SELECT count(*) FROM zdx_claude_sessions WHERE project_id = $1;

-- name: ListClaudeSessionsPaginated :many
SELECT id, project_id, issue_id, session_id, title, created_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CreateClaudeEvent :exec
INSERT INTO zdx_claude_events (session_pk, seq, event_type, event_json)
VALUES ($1, $2, $3, $4);

-- name: ListClaudeEvents :many
SELECT id, session_pk, seq, event_type, event_json, created_at
FROM zdx_claude_events
WHERE session_pk = $1
ORDER BY seq;

-- name: CountClaudeEvents :one
SELECT count(*) FROM zdx_claude_events WHERE session_pk = $1;

-- name: ListClaudeEventsPaginated :many
SELECT id, session_pk, seq, event_type, event_json, created_at
FROM zdx_claude_events
WHERE session_pk = $1
ORDER BY seq
LIMIT $2 OFFSET $3;

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
