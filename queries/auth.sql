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
