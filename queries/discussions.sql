-- name: CreateDiscussion :one
INSERT INTO zdx_discussions (project_id, title, provider, created_by)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, title, provider, claude_session_id, openai_thread_id, status, created_by, created_at, updated_at;

-- name: GetDiscussion :one
SELECT id, project_id, title, provider, claude_session_id, openai_thread_id, status, created_by, created_at, updated_at
FROM zdx_discussions WHERE project_id = $1 AND id = $2;

-- name: ListDiscussions :many
SELECT id, project_id, title, provider, claude_session_id, openai_thread_id, status, created_by, created_at, updated_at
FROM zdx_discussions WHERE project_id = $1 AND ($2::text = 'all' OR status = $2::text)
ORDER BY created_at DESC;

-- name: UpdateDiscussionSession :one
UPDATE zdx_discussions
SET claude_session_id = $3, openai_thread_id = $4, updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, title, provider, claude_session_id, openai_thread_id, status, created_by, created_at, updated_at;

-- name: UpdateDiscussionTitle :exec
UPDATE zdx_discussions SET title = $3, updated_at = NOW() WHERE project_id = $1 AND id = $2;

-- name: UpdateDiscussionStatus :exec
UPDATE zdx_discussions SET status = $3, updated_at = NOW() WHERE project_id = $1 AND id = $2;

-- name: DeleteDiscussion :exec
DELETE FROM zdx_discussions WHERE project_id = $1 AND id = $2;

-- name: CreateDiscussionMessage :one
INSERT INTO zdx_discussion_messages (discussion_id, role, content)
VALUES ($1, $2, $3)
RETURNING id, discussion_id, role, content, created_at;

-- name: ListDiscussionMessages :many
SELECT id, discussion_id, role, content, created_at
FROM zdx_discussion_messages WHERE discussion_id = $1
ORDER BY created_at ASC;

-- name: ListDiscussionsAwaitingResponse :many
-- Active discussions whose most-recent message is from the user — i.e. the
-- assistant has not yet replied. Drives an agent-queue todo so an agent can pick
-- up the dangling thread (typically left over from a failed LLM send).
SELECT d.id, d.title, m.id AS message_id, m.content
FROM zdx_discussions d
JOIN LATERAL (
    SELECT id, role, content
    FROM zdx_discussion_messages
    WHERE discussion_id = d.id
    ORDER BY created_at DESC, id DESC
    LIMIT 1
) m ON TRUE
WHERE d.project_id = $1
  AND d.status = 'active'
  AND m.role = 'user';
