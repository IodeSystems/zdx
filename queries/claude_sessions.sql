-- name: CreateClaudeSession :one
INSERT INTO zdx_claude_sessions (project_id, issue_id, session_id, title, alias)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at;

-- name: GetClaudeSession :one
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions WHERE project_id = $1 AND id = $2;

-- name: GetClaudeSessionBySessionID :one
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions WHERE project_id = $1 AND session_id = $2;

-- name: ListClaudeSessions :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY updated_at DESC;

-- name: CountClaudeSessions :one
SELECT count(*) FROM zdx_claude_sessions WHERE project_id = $1;

-- name: ListClaudeSessionsPaginated :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions
WHERE project_id = $1
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: ListClaudeSessionsByIssue :many
SELECT id, project_id, issue_id, session_id, title, alias, header, summary, status, created_at, updated_at, closed_at
FROM zdx_claude_sessions
WHERE project_id = $1 AND issue_id = $2
ORDER BY updated_at DESC;

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

-- name: ListClaudeEventsPaginated :many
SELECT id, session_pk, seq, event_type, event_json, created_at, agent_id, is_sidechain, agent_type, agent_description
FROM zdx_claude_events
WHERE session_pk = $1
ORDER BY seq DESC
LIMIT $2 OFFSET $3;

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
