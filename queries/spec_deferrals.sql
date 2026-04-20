-- name: AddSpecDeferral :exec
INSERT INTO zdx_spec_deferrals (spec_id, issue_id, note)
VALUES ($1, $2, $3)
ON CONFLICT (spec_id, issue_id) DO UPDATE SET note = EXCLUDED.note;

-- name: RemoveSpecDeferral :exec
DELETE FROM zdx_spec_deferrals WHERE spec_id = $1 AND issue_id = $2;

-- name: ListSpecDeferrals :many
SELECT sd.spec_id, sd.issue_id, sd.note, sd.created_at, i.title AS issue_title, i.status AS issue_status
FROM zdx_spec_deferrals sd
JOIN zdx_issues i ON i.id = sd.issue_id
WHERE sd.spec_id = $1
ORDER BY sd.created_at;

-- name: ListDeferredSpecs :many
SELECT DISTINCT s.id, s.feature_id, s.description, s.kind, s.concern_type
FROM zdx_specs s
JOIN zdx_spec_deferrals sd ON sd.spec_id = s.id
JOIN zdx_issues i ON i.id = sd.issue_id
WHERE i.status = 'open'
ORDER BY s.id;

-- name: ListSpecsWithAllBlockersClosed :many
SELECT s.id, s.feature_id, s.description, s.kind, s.concern_type
FROM zdx_specs s
WHERE EXISTS (SELECT 1 FROM zdx_spec_deferrals sd WHERE sd.spec_id = s.id)
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = s.id AND i.status = 'open'
  )
ORDER BY s.id;

-- name: IsSpecDeferred :one
SELECT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = $1 AND i.status = 'open'
) AS deferred;
