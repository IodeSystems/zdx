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
-- Excludes agent-authored comments (author_alias != '') so agents don't review their own replies.
SELECT EXISTS (
  SELECT 1 FROM zdx_comments c
  WHERE c.project_id = @project_id
    AND c.target_type = @target_type
    AND c.target_id = @target_id
    AND c.author_alias = ''
    AND NOT EXISTS (
      SELECT 1 FROM zdx_comment_reads r
      WHERE r.project_id = c.project_id
        AND r.target_type = c.target_type
        AND r.target_id = c.target_id
        AND r.role = @role
        AND r.last_read_at >= c.created_at
    )
);

-- name: CountUnreadResponsesForUser :one
-- Counts comments on targets where the user has previously commented,
-- excluding the user's own comments, that are newer than the user's last read.
SELECT COUNT(*)::int FROM zdx_comments c
WHERE c.project_id = @project_id
  AND c.author != @author
  AND EXISTS (
    SELECT 1 FROM zdx_comments mine
    WHERE mine.project_id = c.project_id
      AND mine.target_type = c.target_type
      AND mine.target_id = c.target_id
      AND mine.author = @author
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = @role
      AND r.last_read_at >= c.created_at
  );

-- name: ListUnreadResponseThreadsForUser :many
-- metaquery: off
-- Returns distinct threads where the user has commented and others have replied unread.
SELECT
  c.target_type,
  c.target_id,
  COUNT(*)::int AS unread_count,
  MAX(c.created_at) AS last_unread_at
FROM zdx_comments c
WHERE c.project_id = @project_id
  AND c.author != @author
  AND EXISTS (
    SELECT 1 FROM zdx_comments mine
    WHERE mine.project_id = c.project_id
      AND mine.target_type = c.target_type
      AND mine.target_id = c.target_id
      AND mine.author = @author
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = @role
      AND r.last_read_at >= c.created_at
  )
GROUP BY c.target_type, c.target_id
ORDER BY MAX(c.created_at) DESC
LIMIT 50;

-- name: DismissAllUnreadResponsesForUser :exec
-- Marks all unread response threads as read for the user by upserting comment reads.
INSERT INTO zdx_comment_reads (project_id, target_type, target_id, role)
SELECT DISTINCT c.project_id, c.target_type, c.target_id, @role::text
FROM zdx_comments c
WHERE c.project_id = @project_id
  AND c.author != @author
  AND EXISTS (
    SELECT 1 FROM zdx_comments mine
    WHERE mine.project_id = c.project_id
      AND mine.target_type = c.target_type
      AND mine.target_id = c.target_id
      AND mine.author = @author
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = @role
      AND r.last_read_at >= c.created_at
  )
ON CONFLICT (project_id, target_type, target_id, role)
DO UPDATE SET last_read_at = NOW();

-- name: ListStaleUnreadComments :many
-- metaquery: off
-- Returns comments that are unread for the given role and older than the given age threshold.
-- Excludes comments authored by agents (author_alias != '') so agents don't review their own replies.
SELECT c.id, c.project_id, c.target_type, c.target_id, c.author, c.body, c.created_at, c.parent_id, c.author_alias
FROM zdx_comments c
WHERE c.project_id = @project_id
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = @role
      AND r.last_read_at >= c.created_at
  )
  AND c.created_at <= NOW() - make_interval(hours => @age_hours::int)
  AND c.author_alias = ''
ORDER BY c.created_at
LIMIT 20;

-- name: ListIssuesWithUnreadComments :many
-- Returns issues (any status) that have unread comments for the given role.
-- Excludes agent-authored comments (author_alias != '') so agents don't review their own replies.
SELECT DISTINCT i.id, i.title, i.status
FROM zdx_issues i
JOIN zdx_comments c ON c.project_id = i.project_id AND c.target_type = 'issue' AND c.target_id = i.id
WHERE i.project_id = @project_id
  AND c.author_alias = ''
  AND NOT EXISTS (
    SELECT 1 FROM zdx_comment_reads r
    WHERE r.project_id = c.project_id
      AND r.target_type = c.target_type
      AND r.target_id = c.target_id
      AND r.role = @role
      AND r.last_read_at >= c.created_at
  )
ORDER BY i.id;

-- name: GetCommentByID :one
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE id = $1;

-- name: GetCommentsByIDs :many
SELECT id, project_id, target_type, target_id, author, body, created_at, parent_id, author_alias
FROM zdx_comments WHERE id = ANY($1::int[]);

-- name: AddCommentReaction :one
INSERT INTO zdx_comment_reactions (project_id, comment_id, emoji, reactor)
VALUES ($1, $2, $3, $4)
ON CONFLICT (comment_id, emoji, reactor) DO NOTHING
RETURNING id, project_id, comment_id, emoji, reactor, created_at;

-- name: ListCommentReactions :many
SELECT id, project_id, comment_id, emoji, reactor, created_at
FROM zdx_comment_reactions WHERE comment_id = $1
ORDER BY created_at;

-- name: DeleteCommentReaction :exec
DELETE FROM zdx_comment_reactions
WHERE comment_id = $1 AND emoji = $2 AND reactor = $3;

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
