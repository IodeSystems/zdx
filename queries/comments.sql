-- name: AddComment :one
INSERT INTO zdx_comments (project_id, target_type, target_id, author, body, parent_id, author_alias)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias;

-- name: ListComments :many
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE project_id = $1 AND target_type = $2 AND target_id = $3
ORDER BY created_at;


-- name: ListCommentsByAuthor :many
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE project_id = $1 AND author = $2
ORDER BY created_at DESC;

-- name: GetCommentByID :one
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE id = $1;

-- name: GetCommentsByIDs :many
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE id = ANY($1::int[]);

-- name: AddRevision :exec
INSERT INTO zdx_revisions (project_id, target_type, target_id, field, old_val, new_val, agent, session_id, user_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9);

-- name: ListRevisions :many
SELECT id, project_id, target_type, target_id, field, old_val, new_val, agent, session_id, user_id, created_at
FROM zdx_revisions WHERE project_id = $1 AND target_type = $2 AND target_id = $3
ORDER BY created_at;


-- name: ListRevisionsByTarget :many
SELECT id, project_id, target_type, target_id, field, old_val, new_val, agent, session_id, user_id, created_at
FROM zdx_revisions WHERE target_type = @target_type AND target_id = @target_id
ORDER BY created_at DESC;

-- name: CountRevisionsBySession :one
-- Count how many revisions were recorded by a given agent session.
-- Used by /api/dx/solo/release to detect sessions that exited cleanly but
-- didn't apply any durable mutation — those release without marking resolved.
SELECT count(*) FROM zdx_revisions WHERE project_id = $1 AND session_id = $2;
