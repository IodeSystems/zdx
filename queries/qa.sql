-- name: InsertQuestion :one
INSERT INTO zdx_questions (project_id, category, question)
VALUES ($1, $2, $3)
RETURNING id, project_id, category, question, answer, created_at, updated_at;

-- name: AnswerQuestion :one
UPDATE zdx_questions
SET answer     = $3,
    updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, category, question, answer, created_at, updated_at;

-- name: ListQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at
FROM zdx_questions
WHERE project_id = $1
ORDER BY created_at;

-- name: GetQuestion :one
SELECT id, project_id, category, question, answer, created_at, updated_at
FROM zdx_questions
WHERE project_id = $1 AND id = $2;
