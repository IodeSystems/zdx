-- name: CreateSessionAuditEvent :exec
INSERT INTO zdx_session_audit_events (session_pk, agent_id, todo_id, turn_id, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListSessionAuditEvents :many
SELECT id, session_pk, agent_id, todo_id, turn_id, event_type, payload, occurred_at
FROM zdx_session_audit_events
WHERE session_pk = $1
ORDER BY occurred_at;

-- name: ListAgentAuditEvents :many
SELECT id, session_pk, agent_id, todo_id, turn_id, event_type, payload, occurred_at
FROM zdx_session_audit_events
WHERE agent_id = $1
ORDER BY occurred_at
LIMIT 200;
