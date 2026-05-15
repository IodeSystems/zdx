-- name: UpsertEnvAgentHeartbeat :one
-- Insert-or-update one zdx_env_agents row keyed by (env_id, agent_id).
-- Called on the initial WS handshake AND on every subsequent heartbeat;
-- last_heartbeat_at always advances to now() so `dx env list` can grade
-- env liveness off the column.
INSERT INTO zdx_env_agents (env_id, agent_id, hostname, version, os, deployed_commit, uptime_secs)
VALUES (@env_id, @agent_id, @hostname, @version, @os, @deployed_commit, @uptime_secs)
ON CONFLICT (env_id, agent_id) DO UPDATE SET
    version           = EXCLUDED.version,
    os                = EXCLUDED.os,
    deployed_commit   = EXCLUDED.deployed_commit,
    uptime_secs       = EXCLUDED.uptime_secs,
    last_heartbeat_at = now()
RETURNING *;

-- name: ListEnvAgentsForEnv :many
SELECT * FROM zdx_env_agents
WHERE env_id = $1
ORDER BY last_heartbeat_at DESC;

-- name: GetEnvAgentByAgentID :one
SELECT * FROM zdx_env_agents
WHERE env_id = $1 AND agent_id = $2;
