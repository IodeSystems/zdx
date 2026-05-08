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

-- name: ListTargetsWithComments :many
SELECT DISTINCT target_type, target_id
FROM zdx_comments
WHERE project_id = $1
ORDER BY target_type, target_id;

-- name: ListTargetsWithUnreadComments :many
-- Targets with a comment newer than the seen high-water mark. Used by the
-- agent-queue regenerator to emit the synthetic read:comments todo only when
-- there is actually unread content to surface — otherwise the todo would
-- regenerate every loop iteration even after the agent claims and "reads"
-- it, producing the cycle observed in IS-1040.
--
-- IS-687 / TK-1455: only surface a target when the LATEST unread comment
-- is from a non-agent author (author_alias = ''). If the agent replied last,
-- the target is not surfaced — the agent has already done its job and should
-- not be asked to read its own reply.
WITH unread AS (
    SELECT c.target_type, c.target_id, c.created_at, c.author_alias
    FROM zdx_comments c
    LEFT JOIN zdx_target_comments_seen s
           ON s.project_id  = c.project_id
          AND s.target_type = c.target_type
          AND s.target_id   = c.target_id
    WHERE c.project_id = $1
      AND c.id > COALESCE(s.last_comment_id, 0)
),
latest_unread AS (
    SELECT DISTINCT ON (target_type, target_id)
        target_type, target_id, author_alias
    FROM unread
    ORDER BY target_type, target_id, created_at DESC
)
SELECT target_type, target_id
FROM latest_unread
WHERE author_alias = ''
ORDER BY target_type, target_id;

-- name: MarkTargetCommentsSeen :exec
-- Advance the seen high-water mark for one target to the current max comment
-- id. Idempotent: replaying with no new comments leaves the watermark put.
-- Called when an agent resolves a read:comments synthetic todo.
INSERT INTO zdx_target_comments_seen (project_id, target_type, target_id, last_comment_id, seen_at)
SELECT @project_id::int, @target_type::text, @target_id::text, COALESCE(MAX(id), 0), NOW()
FROM zdx_comments
WHERE project_id = @project_id::int
  AND target_type = @target_type::text
  AND target_id   = @target_id::text
ON CONFLICT (project_id, target_type, target_id) DO UPDATE SET
    last_comment_id = GREATEST(zdx_target_comments_seen.last_comment_id, EXCLUDED.last_comment_id),
    seen_at         = NOW();

-- name: ListFeaturesWithPendingComments :many
-- Returns feature target_ids where the most-recent comment has no author_alias
-- (human posted last and the agent has not yet replied).
WITH last_comment AS (
    SELECT DISTINCT ON (target_id) target_id, author_alias
    FROM zdx_comments
    WHERE project_id = $1 AND target_type = 'feature'
    ORDER BY target_id, created_at DESC
)
SELECT target_id FROM last_comment WHERE author_alias = '';

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
-- Used by /api/dx/agent/release to detect sessions that exited cleanly but
-- didn't apply any durable mutation — those release without marking resolved.
SELECT count(*) FROM zdx_revisions WHERE project_id = $1 AND session_id = $2;
