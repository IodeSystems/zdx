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
