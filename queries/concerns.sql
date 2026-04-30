-- name: ListConcerns :many
SELECT id, project_id, name, description, created_at
FROM zdx_concerns WHERE project_id = $1 ORDER BY name;

-- name: GetConcernByName :one
SELECT id, project_id, name, description, created_at
FROM zdx_concerns WHERE project_id = $1 AND name = $2;

-- name: CreateConcern :one
INSERT INTO zdx_concerns (project_id, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id, project_id, name, description, created_at;

-- name: LinkConcernFeature :exec
INSERT INTO zdx_concern_features (concern_id, feature_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkConcernFeature :exec
DELETE FROM zdx_concern_features WHERE concern_id = $1 AND feature_id = $2;

-- name: LinkConcernSpec :exec
INSERT INTO zdx_concern_specs (concern_id, spec_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkConcernSpec :exec
DELETE FROM zdx_concern_specs WHERE concern_id = $1 AND spec_id = $2;

-- name: LinkConcernIssue :exec
INSERT INTO zdx_concern_issues (concern_id, issue_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkConcernIssue :exec
DELETE FROM zdx_concern_issues WHERE concern_id = $1 AND issue_id = $2;

-- name: LinkConcernPattern :exec
INSERT INTO zdx_concern_patterns (concern_id, pattern_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkConcernPattern :exec
DELETE FROM zdx_concern_patterns WHERE concern_id = $1 AND pattern_id = $2;

-- name: ListConcernsForFeature :many
SELECT c.id, c.project_id, c.name, c.description, c.created_at
FROM zdx_concerns c
JOIN zdx_concern_features cf ON cf.concern_id = c.id
WHERE cf.feature_id = $1
ORDER BY c.name;

-- name: ListConcernsForSpec :many
SELECT c.id, c.project_id, c.name, c.description, c.created_at
FROM zdx_concerns c
JOIN zdx_concern_specs cs ON cs.concern_id = c.id
WHERE cs.spec_id = $1
ORDER BY c.name;
