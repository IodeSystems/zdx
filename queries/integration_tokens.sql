-- name: CreateIntegrationToken :one
INSERT INTO zdx_integration_token (project_id, component, name, token_hash, token_prefix, capabilities)
VALUES (sqlc.narg(project_id), sqlc.narg(component), @name, @token_hash, @token_prefix, COALESCE(sqlc.narg(capabilities)::text[], ARRAY['ingest:logs']::text[]))
RETURNING id, project_id, component, name, token_prefix, capabilities, created_at, revoked_at;

-- name: GetIntegrationTokenByHash :one
SELECT id, project_id, component, name, token_prefix, capabilities, created_at, revoked_at
FROM zdx_integration_token
WHERE token_hash = $1;

-- name: ListIntegrationTokens :many
SELECT id, project_id, component, name, token_prefix, capabilities, created_at, revoked_at
FROM zdx_integration_token
WHERE (sqlc.narg(project_id)::int IS NULL OR project_id = sqlc.narg(project_id))
ORDER BY created_at DESC;

-- name: RevokeIntegrationToken :exec
UPDATE zdx_integration_token SET revoked_at = NOW() WHERE id = $1;

-- name: DeleteIntegrationToken :exec
DELETE FROM zdx_integration_token WHERE id = $1;
