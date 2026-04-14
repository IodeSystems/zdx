-- name: ListProposals :many
SELECT id, project_id, journal_entry_id, title, context, status, priority, filed_issue_id, created_at
FROM zdx_proposals WHERE project_id = $1 ORDER BY priority DESC, created_at DESC;

-- name: ListProposalsByStatus :many
SELECT id, project_id, journal_entry_id, title, context, status, priority, filed_issue_id, created_at
FROM zdx_proposals WHERE project_id = $1 AND status = $2 ORDER BY priority DESC, created_at DESC;

-- name: GetProposal :one
SELECT id, project_id, journal_entry_id, title, context, status, priority, filed_issue_id, created_at
FROM zdx_proposals WHERE id = $1;

-- name: CreateProposal :one
INSERT INTO zdx_proposals (project_id, journal_entry_id, title, context, priority)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, journal_entry_id, title, context, status, priority, filed_issue_id, created_at;

-- name: UpdateProposalStatus :exec
UPDATE zdx_proposals SET status = $2 WHERE id = $1;

-- name: FileProposal :exec
UPDATE zdx_proposals SET status = 'filed', filed_issue_id = $2 WHERE id = $1;
