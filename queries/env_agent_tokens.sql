-- name: CreateEnvAgentToken :one
-- Mint a new env-agent token. token_hash is the SHA-256 hex of the plaintext
-- (which is returned once to the caller and never persisted). scopes encodes
-- which dx-envd capabilities the bearer may invoke (e.g. {"deploy:apply",
-- "schema:dump"}).
INSERT INTO zdx_env_agent_tokens (env_id, token_hash, scopes, name)
VALUES (@env_id, @token_hash, @scopes, @name)
RETURNING *;

-- name: GetEnvAgentTokenByHash :one
-- Middleware hot path: resolve a presented token to its env_id + scopes.
-- Filters out revoked rows so a revoked token looks identical to "no such
-- token" to the caller.
SELECT id, env_id, scopes, name
FROM zdx_env_agent_tokens
WHERE token_hash = $1
  AND revoked_at IS NULL;

-- name: ListEnvAgentTokensByEnv :many
-- Admin/UI listing of live tokens for an env. token_hash is intentionally
-- omitted so the listing is safe to log.
SELECT id, name, scopes, created_at, last_used_at
FROM zdx_env_agent_tokens
WHERE env_id = $1
  AND revoked_at IS NULL
ORDER BY created_at DESC;

-- name: RevokeEnvAgentToken :exec
UPDATE zdx_env_agent_tokens
SET revoked_at = now()
WHERE id = $1
  AND revoked_at IS NULL;

-- name: TouchEnvAgentTokenLastUsed :exec
-- Fire-and-forget update from the auth middleware after a successful lookup.
UPDATE zdx_env_agent_tokens
SET last_used_at = now()
WHERE id = $1;
