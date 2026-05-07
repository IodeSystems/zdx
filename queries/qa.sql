-- name: InsertQuestion :one
INSERT INTO zdx_questions (project_id, category, question, parent_question_id, owner_user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id;

-- name: AnswerQuestion :one
UPDATE zdx_questions
SET answer     = $3,
    updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id;

-- name: ListQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = $1
ORDER BY created_at;


-- name: GetQuestion :one
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = $1 AND id = $2;

-- name: ListChildQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = $1 AND parent_question_id = $2
ORDER BY created_at;

-- name: ListUnansweredQuestions :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = $1 AND answer IS NULL
ORDER BY created_at;

-- name: ListQuestionsByOwner :many
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = $1 AND owner_user_id = $2 AND answer IS NULL
ORDER BY created_at;

-- name: UpdateQuestionOwner :one
UPDATE zdx_questions
SET owner_user_id = $3,
    updated_at    = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id;

-- name: SearchQuestions :many
-- metaquery: off
SELECT id, project_id, category, question, answer, created_at, updated_at, parent_question_id, owner_user_id
FROM zdx_questions
WHERE project_id = @project_id
  AND (question ILIKE '%' || @query::text || '%' OR answer ILIKE '%' || @query::text || '%')
ORDER BY created_at DESC
LIMIT 10;
