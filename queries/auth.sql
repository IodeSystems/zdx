-- name: GetApiKeyByToken :one
SELECT id, user_id, token, name, last_used_at, created_at
FROM zdx_api_keys WHERE token = $1;

-- name: TouchApiKey :exec
UPDATE zdx_api_keys SET last_used_at = NOW() WHERE id = $1;

-- name: CountApiKeys :one
SELECT COUNT(*)::int FROM zdx_api_keys;

-- name: CreateUser :one
INSERT INTO zdx_users (email, name, password_hash, role)
VALUES ($1, $2, '', 'admin')
RETURNING id, email, name, role, created_at;

-- name: CreateUserWithPassword :one
INSERT INTO zdx_users (email, name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING id, email, name, role, created_at;

-- name: GetUserByEmail :one
SELECT id, email, name, password_hash, role, created_at FROM zdx_users WHERE email = $1;

-- name: GetUserByID :one
SELECT id, email, name, role, created_at FROM zdx_users WHERE id = $1;

-- name: GetApiKeyUserRole :one
SELECT u.role FROM zdx_api_keys k JOIN zdx_users u ON u.id = k.user_id WHERE k.token = $1;

-- name: CreateApiKey :one
INSERT INTO zdx_api_keys (user_id, token, name)
VALUES ($1, $2, $3)
RETURNING id, user_id, token, name, last_used_at, created_at;

-- name: ListUsers :many
SELECT id, email, name, role, created_at FROM zdx_users ORDER BY created_at DESC;

-- name: SearchUsers :many
SELECT id, email, name, role, created_at FROM zdx_users
WHERE email ILIKE '%' || @q::text || '%' OR name ILIKE '%' || @q::text || '%'
ORDER BY created_at DESC;

-- name: UpdateUserRole :exec
UPDATE zdx_users SET role = $2 WHERE id = $1;

-- name: ListInvites :many
SELECT id, email, token, invited_by, expires_at, used_at, created_at
FROM zdx_invites ORDER BY created_at DESC;

-- name: CreateInvite :one
INSERT INTO zdx_invites (email, token, invited_by)
VALUES ($1, $2, $3)
RETURNING id, email, token, invited_by, expires_at, used_at, created_at;

-- name: DeleteInvite :exec
DELETE FROM zdx_invites WHERE id = $1;

-- name: GetInviteByToken :one
SELECT id, email, token, invited_by, expires_at, used_at, created_at
FROM zdx_invites WHERE token = $1 AND used_at IS NULL AND expires_at > NOW();

-- name: MarkInviteUsed :exec
UPDATE zdx_invites SET used_at = NOW() WHERE id = $1;

-- name: ListApiKeysByUser :many
SELECT id, name, last_used_at, created_at
FROM zdx_api_keys WHERE user_id = $1 ORDER BY created_at DESC;

-- name: DeleteApiKey :exec
DELETE FROM zdx_api_keys WHERE id = $1 AND user_id = $2;
