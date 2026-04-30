-- name: RegisterAgent :one
INSERT INTO zdx_agents (id, project_id, session_id, worktree_path, worktree_branch, pid, status, task_group, compose_project, server_port, database_url, valkey_url)
VALUES (@id, @project_id, @session_id, @worktree_path, @worktree_branch, @pid, @status, @task_group, @compose_project, @server_port, @database_url, @valkey_url)
ON CONFLICT (id) DO UPDATE SET
    session_id = EXCLUDED.session_id,
    worktree_path = EXCLUDED.worktree_path,
    worktree_branch = EXCLUDED.worktree_branch,
    pid = EXCLUDED.pid,
    status = EXCLUDED.status,
    task_group = EXCLUDED.task_group,
    compose_project = EXCLUDED.compose_project,
    server_port = EXCLUDED.server_port,
    database_url = EXCLUDED.database_url,
    valkey_url = EXCLUDED.valkey_url,
    last_heartbeat = NOW()
RETURNING *;

-- name: GetAgent :one
SELECT * FROM zdx_agents WHERE id = $1;

-- name: ListAgentsByProject :many
SELECT * FROM zdx_agents WHERE project_id = $1 ORDER BY last_heartbeat DESC;

-- name: UpdateAgentHeartbeat :exec
UPDATE zdx_agents SET last_heartbeat = NOW() WHERE id = $1;

-- name: DeleteAgent :exec
DELETE FROM zdx_agents WHERE id = $1;

-- name: ReapStaleAgents :many
DELETE FROM zdx_agents
WHERE last_heartbeat < NOW() - @stale_threshold::interval
RETURNING *;

-- name: UpdateAgentStatus :exec
UPDATE zdx_agents SET status = @status WHERE id = @id;

-- name: MarkAgentDisconnected :exec
-- Set disconnect_at=now() and flip to 'disconnected' unless already paused/draining.
UPDATE zdx_agents
SET status       = CASE WHEN status IN ('paused', 'draining') THEN status ELSE 'disconnected' END,
    disconnect_at = NOW()
WHERE id = $1;

-- name: MarkAgentConnected :exec
-- Clear disconnect_at on reconnect. Keep 'paused' if the operator set it before reconnect.
UPDATE zdx_agents
SET status       = CASE WHEN status = 'paused' THEN 'paused' ELSE 'active' END,
    disconnect_at = NULL
WHERE id = $1;
