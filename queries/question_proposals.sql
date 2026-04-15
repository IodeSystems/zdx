-- name: InsertQuestionProposal :one
INSERT INTO zdx_question_proposals (project_id, question_id, question_type, title, context)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, question_id, question_type, title, context, status, denied_reason, created_issue_id, created_at, updated_at;

-- name: GetQuestionProposal :one
SELECT id, project_id, question_id, question_type, title, context, status, denied_reason, created_issue_id, created_at, updated_at
FROM zdx_question_proposals WHERE project_id = $1 AND id = $2;

-- name: ListQuestionProposalsByQuestion :many
SELECT id, project_id, question_id, question_type, title, context, status, denied_reason, created_issue_id, created_at, updated_at
FROM zdx_question_proposals
WHERE project_id = $1 AND question_id = $2 AND question_type = $3
ORDER BY created_at;

-- name: AcceptQuestionProposal :one
UPDATE zdx_question_proposals
SET status = 'accepted', created_issue_id = $3, updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, question_id, question_type, title, context, status, denied_reason, created_issue_id, created_at, updated_at;

-- name: DenyQuestionProposal :one
UPDATE zdx_question_proposals
SET status = 'denied', denied_reason = $3, updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, question_id, question_type, title, context, status, denied_reason, created_issue_id, created_at, updated_at;

-- name: CountQuestionProposalsByQuestion :one
SELECT count(*) FROM zdx_question_proposals
WHERE project_id = $1 AND question_id = $2 AND question_type = $3;
