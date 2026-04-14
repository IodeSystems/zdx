-- name: CreateClaudeSession :one
INSERT INTO zdx_claude_sessions (project_id, issue_id, session_id, title, alias)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at;

-- name: GetClaudeSession :one
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at
FROM zdx_claude_sessions WHERE project_id = $1 AND id = $2;

-- name: GetClaudeSessionBySessionID :one
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at
FROM zdx_claude_sessions WHERE project_id = $1 AND session_id = $2;

-- name: ListClaudeSessions :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: CountClaudeSessions :one
SELECT count(*) FROM zdx_claude_sessions WHERE project_id = $1;

-- name: ListClaudeSessionsPaginated :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateClaudeSessionSummary :exec
UPDATE zdx_claude_sessions
SET header = $3, summary = $4, status = $5
WHERE project_id = $1 AND id = $2;

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

-- name: ListClaudeEventsPaginated :many
SELECT id, session_pk, seq, event_type, event_json, created_at, agent_id, is_sidechain, agent_type, agent_description
FROM zdx_claude_events
WHERE session_pk = $1
ORDER BY seq DESC
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
