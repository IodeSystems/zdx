-- name: CreateCodeRef :one
INSERT INTO zdx_code_refs (project_id, file_path, git_hash, line_start, line_end, note)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, project_id, file_path, git_hash, line_start, line_end, note, created_at;

-- name: GetCodeRef :one
SELECT id, project_id, file_path, git_hash, line_start, line_end, note, created_at
FROM zdx_code_refs
WHERE project_id = $1 AND id = $2;

-- name: DeleteCodeRef :exec
DELETE FROM zdx_code_refs WHERE project_id = $1 AND id = $2;

-- name: AttachCodeRefToIssue :exec
INSERT INTO zdx_issue_code_refs (issue_id, code_ref_id)
VALUES ($1, $2)
ON CONFLICT (issue_id, code_ref_id) DO NOTHING;

-- name: DetachCodeRefFromIssue :exec
DELETE FROM zdx_issue_code_refs WHERE issue_id = $1 AND code_ref_id = $2;

-- name: ListCodeRefsByIssue :many
SELECT r.id, r.project_id, r.file_path, r.git_hash, r.line_start, r.line_end, r.note, r.created_at
FROM zdx_code_refs r
JOIN zdx_issue_code_refs ir ON ir.code_ref_id = r.id
WHERE ir.issue_id = $1
ORDER BY r.created_at;

-- name: AttachCodeRefToTask :exec
INSERT INTO zdx_task_code_refs (task_id, code_ref_id)
VALUES ($1, $2)
ON CONFLICT (task_id, code_ref_id) DO NOTHING;

-- name: DetachCodeRefFromTask :exec
DELETE FROM zdx_task_code_refs WHERE task_id = $1 AND code_ref_id = $2;

-- name: ListCodeRefsByTask :many
SELECT r.id, r.project_id, r.file_path, r.git_hash, r.line_start, r.line_end, r.note, r.created_at
FROM zdx_code_refs r
JOIN zdx_task_code_refs tr ON tr.code_ref_id = r.id
WHERE tr.task_id = $1
ORDER BY r.created_at;

-- name: AttachCodeRefToTest :exec
INSERT INTO zdx_test_code_refs (test_id, code_ref_id)
VALUES ($1, $2)
ON CONFLICT (test_id, code_ref_id) DO NOTHING;

-- name: DetachCodeRefFromTest :exec
DELETE FROM zdx_test_code_refs WHERE test_id = $1 AND code_ref_id = $2;

-- name: ListCodeRefsByTest :many
SELECT r.id, r.project_id, r.file_path, r.git_hash, r.line_start, r.line_end, r.note, r.created_at
FROM zdx_code_refs r
JOIN zdx_test_code_refs tr ON tr.code_ref_id = r.id
WHERE tr.test_id = $1
ORDER BY r.created_at;

-- name: AttachCodeRefToSpec :exec
INSERT INTO zdx_spec_code_refs (spec_id, code_ref_id)
VALUES ($1, $2)
ON CONFLICT (spec_id, code_ref_id) DO NOTHING;

-- name: DetachCodeRefFromSpec :exec
DELETE FROM zdx_spec_code_refs WHERE spec_id = $1 AND code_ref_id = $2;

-- name: ListCodeRefsBySpec :many
SELECT r.id, r.project_id, r.file_path, r.git_hash, r.line_start, r.line_end, r.note, r.created_at
FROM zdx_code_refs r
JOIN zdx_spec_code_refs sr ON sr.code_ref_id = r.id
WHERE sr.spec_id = $1
ORDER BY r.created_at;

