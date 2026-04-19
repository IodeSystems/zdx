-- name: ListMaturityQuestions :many
SELECT key, prompt, answer_type, priority_hint, applicable_classifications
FROM zdx_maturity_questions
ORDER BY priority_hint, key;

-- name: GetMaturityAnswer :one
SELECT id, project_id, question_key, answer, answered_by, answered_at
FROM zdx_maturity_answers
WHERE project_id = $1 AND question_key = $2;

-- name: ListMaturityAnswers :many
SELECT id, project_id, question_key, answer, answered_by, answered_at
FROM zdx_maturity_answers
WHERE project_id = $1
ORDER BY question_key;

-- name: UpsertMaturityAnswer :one
INSERT INTO zdx_maturity_answers (project_id, question_key, answer, answered_by, answered_at)
VALUES (@project_id, @question_key, @answer, @answered_by, now())
ON CONFLICT (project_id, question_key)
DO UPDATE SET
    answer = EXCLUDED.answer,
    answered_by = EXCLUDED.answered_by,
    answered_at = now()
RETURNING id, project_id, question_key, answer, answered_by, answered_at;
