-- name: CreateChurnHint :one
INSERT INTO zdx_churn_hints (project_id, title, summary)
VALUES ($1, $2, $3)
RETURNING id, project_id, title, summary, status, created_at, last_active_at, closed_at, closed_by_ref;

-- name: GetChurnHint :one
SELECT id, project_id, title, summary, status, created_at, last_active_at, closed_at, closed_by_ref
FROM zdx_churn_hints
WHERE project_id = $1 AND id = $2;

-- name: ListOpenChurnHintsByProject :many
SELECT id, project_id, title, summary, status, created_at, last_active_at, closed_at, closed_by_ref
FROM zdx_churn_hints
WHERE project_id = $1 AND status = 'open'
ORDER BY last_active_at DESC, id DESC;

-- name: UpdateChurnHintLastActive :exec
UPDATE zdx_churn_hints
SET last_active_at = NOW()
WHERE id = $1;

-- name: CloseChurnHint :exec
UPDATE zdx_churn_hints
SET status = $2,
    closed_at = NOW(),
    closed_by_ref = $3
WHERE id = $1;

-- name: AppendChurnHintRef :one
INSERT INTO zdx_churn_hint_refs (hint_id, ref_type, ref_id, observation)
VALUES ($1, $2, $3, $4)
RETURNING id, hint_id, ref_type, ref_id, observation, created_at;

-- name: CountRefsByHint :one
SELECT COUNT(*)::int AS ref_count
FROM zdx_churn_hint_refs
WHERE hint_id = $1;

-- name: AppendChurnHintResolution :one
INSERT INTO zdx_churn_hint_resolutions (hint_id, resolution_type, resolution_id)
VALUES ($1, $2, $3)
RETURNING id, hint_id, resolution_type, resolution_id, created_at;

-- name: ListResolutionsByHint :many
SELECT id, hint_id, resolution_type, resolution_id, created_at
FROM zdx_churn_hint_resolutions
WHERE hint_id = $1
ORDER BY created_at, id;
