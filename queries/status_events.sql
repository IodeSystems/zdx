-- name: InsertStatusEvent :exec
INSERT INTO zdx_status_events (project_id, target_type, target_id, from_status, to_status, agent_id, session_id, user_id)
VALUES (@project_id, @target_type, @target_id, @from_status, @to_status, @agent_id, @session_id, @user_id);

-- name: ListStatusEventsByTarget :many
SELECT e.id, e.project_id, e.target_type, e.target_id, e.from_status, e.to_status, e.agent_id, e.session_id, e.user_id, e.created_at
FROM zdx_status_events e
WHERE e.target_type = @target_type AND e.target_id = @target_id
ORDER BY e.created_at DESC;
