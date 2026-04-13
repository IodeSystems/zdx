-- name: AddComment :one
INSERT INTO zdx_comments (project_id, target_type, target_id, author, body)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, target_type, target_id, author, body, created_at;

-- name: ListComments :many
SELECT id, project_id, target_type, target_id, author, body, created_at
FROM zdx_comments WHERE project_id = $1 AND target_type = $2 AND target_id = $3
ORDER BY created_at;

-- name: UpsertCommentRead :exec
INSERT INTO zdx_comment_reads (project_id, target_type, target_id, role)
VALUES (@project_id, @target_type, @target_id, @role)
ON CONFLICT (project_id, target_type, target_id, role)
DO UPDATE SET last_read_at = NOW();

-- name: GetCommentRead :one
SELECT last_read_at FROM zdx_comment_reads
WHERE project_id = $1 AND target_type = $2 AND target_id = $3 AND role = $4;

-- name: CountUnreadForRole :one
SELECT COUNT(*)::int FROM zdx_comments c
WHERE c.project_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = $2
      AND r.last_read_at >= c.created_at
  );

-- name: HasUnreadCommentsForTarget :one
SELECT EXISTS (
  SELECT 1 FROM zdx_comments c
  WHERE c.project_id = @project_id
    AND c.target_type = @target_type
    AND c.target_id = @target_id
    AND NOT EXISTS (
      SELECT 1 FROM zdx_comment_reads r
      WHERE r.project_id = c.project_id
        AND r.target_type = c.target_type
        AND r.target_id = c.target_id
        AND r.role = @role
        AND r.last_read_at >= c.created_at
    )
);

-- name: AddRevision :exec
INSERT INTO zdx_revisions (project_id, target_type, target_id, field, old_val, new_val, agent)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: ListRevisions :many
SELECT id, project_id, target_type, target_id, field, old_val, new_val, agent, created_at
FROM zdx_revisions WHERE project_id = $1 AND target_type = $2 AND target_id = $3
ORDER BY created_at;
