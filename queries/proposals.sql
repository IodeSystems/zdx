-- name: CreateProposal :one
INSERT INTO zdx_proposals (project_id, title, value, body, source_type, source_ref, created_by)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value;

-- name: GetProposal :one
SELECT id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value
FROM zdx_proposals WHERE project_id = $1 AND id = $2;

-- name: ListProposals :many
SELECT id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value
FROM zdx_proposals WHERE project_id = $1 AND ($2::text = 'all' OR status = $2::text)
ORDER BY created_at DESC;

-- name: UpdateProposal :one
UPDATE zdx_proposals
SET title = $3, value = $4, body = $5, updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value;

-- name: UpdateProposalStatus :one
UPDATE zdx_proposals
SET status = $3, snoozed_until = $4, approved_issue_id = $5, updated_at = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value;

-- name: CreateProposalVersion :one
INSERT INTO zdx_proposal_versions (proposal_id, body, edited_by)
VALUES ($1, $2, $3)
RETURNING id, proposal_id, body, edited_by, edited_at;

-- name: ListProposalVersions :many
SELECT id, proposal_id, body, edited_by, edited_at
FROM zdx_proposal_versions WHERE proposal_id = $1
ORDER BY edited_at DESC;

-- name: SearchProposals :many
-- metaquery: off
SELECT id, project_id, title, body, source_type, source_ref, status, snoozed_until, created_by, created_at, updated_at, approved_issue_id, value
FROM zdx_proposals
WHERE project_id = @project_id
  AND status NOT IN ('rejected', 'approved')
  AND (title ILIKE '%' || @query::text || '%' OR body ILIKE '%' || @query::text || '%')
ORDER BY created_at DESC
LIMIT 10;
