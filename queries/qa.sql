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

-- name: LinkQuestionTask :exec
INSERT INTO zdx_question_tasks (question_id, task_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkQuestionTask :exec
DELETE FROM zdx_question_tasks
WHERE question_id = $1 AND task_id = $2;

-- name: ListTasksByQuestion :many
SELECT t.id, t.issue, t.title, t.text, t.status, t.feature, t.reason,
       t.created_at, t.updated_at, t.test_plan, t.test_refs, t.task_group, t.target_branch
FROM zdx_tasks t
JOIN zdx_question_tasks qt ON qt.task_id = t.id
WHERE qt.question_id = $1
ORDER BY qt.created_at;

-- name: ListQuestionsByTask :many
SELECT q.id, q.project_id, q.category, q.question, q.answer, q.created_at, q.updated_at, q.parent_question_id, q.owner_user_id
FROM zdx_questions q
JOIN zdx_question_tasks qt ON qt.question_id = q.id
WHERE qt.task_id = $1
ORDER BY qt.created_at;
