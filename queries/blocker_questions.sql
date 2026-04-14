-- name: InsertBlockerQuestion :one
INSERT INTO zdx_blocker_questions (project_id, target_type, target_id, context, choices)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at;

-- name: GetBlockerQuestion :one
SELECT id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at
FROM zdx_blocker_questions WHERE project_id = $1 AND id = $2;

-- name: ListBlockerQuestionsByTarget :many
SELECT id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at
FROM zdx_blocker_questions
WHERE project_id = $1 AND target_type = $2 AND target_id = $3
ORDER BY created_at;

-- name: ListPendingBlockerQuestions :many
SELECT id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at
FROM zdx_blocker_questions
WHERE project_id = $1 AND status = 'pending'
ORDER BY created_at;

-- name: ListBlockerQuestions :many
SELECT id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at
FROM zdx_blocker_questions
WHERE project_id = $1
ORDER BY created_at DESC;

-- name: CountBlockerQuestions :one
SELECT count(*) FROM zdx_blocker_questions WHERE project_id = $1;

-- name: ListBlockerQuestionsPaginated :many
SELECT id, project_id, target_type, target_id, context, choices, answer, answered_by, status, created_at, answered_at
FROM zdx_blocker_questions
WHERE project_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: AnswerBlockerQuestion :exec
UPDATE zdx_blocker_questions
SET answer = $3, answered_by = $4, status = 'answered', answered_at = NOW()
WHERE project_id = $1 AND id = $2;
