-- name: InsertQuestion :one
INSERT INTO zdx_questions (project_id, category, question, parent_question_id)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, category, question, answer, created_at, updated_at, parent_question_id;

-- name: AnswerQuestion :one
UPDATE zdx_questions
SET answer     = $3,
    updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, category, question, answer, created_at, updated_at, parent_question_id;

-- name: ListQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id
FROM zdx_questions
WHERE project_id = $1
ORDER BY created_at;

-- name: CountQuestions :one
SELECT count(*) FROM zdx_questions WHERE project_id = $1;

-- name: ListQuestionsPaginated :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id
FROM zdx_questions
WHERE project_id = $1
ORDER BY created_at
LIMIT $2 OFFSET $3;

-- name: GetQuestion :one
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id
FROM zdx_questions
WHERE project_id = $1 AND id = $2;

-- name: ListChildQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id
FROM zdx_questions
WHERE project_id = $1 AND parent_question_id = $2
ORDER BY created_at;
