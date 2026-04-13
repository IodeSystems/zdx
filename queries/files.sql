-- name: CreateFile :one
INSERT INTO zdx_files (provider, path, mime_type, size_bytes)
VALUES ($1, $2, $3, $4)
RETURNING id, provider, path, mime_type, size_bytes, created_at;

-- name: AttachFileToIssue :exec
INSERT INTO zdx_issue_files (issue_id, file_id, kind)
VALUES ($1, $2, $3)
ON CONFLICT (issue_id, file_id) DO NOTHING;

-- name: GetIssueFiles :many
SELECT f.id, f.provider, f.path, f.mime_type, f.size_bytes, f.created_at, isf.kind
FROM zdx_files f
JOIN zdx_issue_files isf ON isf.file_id = f.id
WHERE isf.issue_id = $1
ORDER BY f.created_at;

-- name: GetFile :one
SELECT id, provider, path, mime_type, size_bytes, created_at FROM zdx_files WHERE id = $1;
