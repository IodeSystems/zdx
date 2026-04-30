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

-- name: ListConcernsForIssue :many
SELECT c.id, c.project_id, c.name, c.description, c.created_at
FROM zdx_concerns c
JOIN zdx_concern_issues ci ON ci.concern_id = c.id
WHERE ci.issue_id = $1
ORDER BY c.name;

-- name: GetConcernDoctorState :one
SELECT
    (SELECT COUNT(*) FROM zdx_concerns cc WHERE cc.project_id = p.id)::int                                AS concern_count,
    (SELECT COUNT(DISTINCT f.id)
     FROM zdx_features f
     WHERE f.project_id = p.id
       AND EXISTS (SELECT 1 FROM zdx_specs WHERE feature_id = f.id)
       AND EXISTS (
           SELECT 1 FROM zdx_concern_features cf
           JOIN zdx_concerns c2 ON c2.id = cf.concern_id
           WHERE cf.feature_id = f.id AND c2.project_id = p.id
       ))::int                                                                                              AS features_with_concerns,
    (SELECT COUNT(DISTINCT c3.id)
     FROM zdx_concerns c3
     WHERE c3.project_id = p.id
       AND EXISTS (SELECT 1 FROM zdx_concern_specs WHERE concern_id = c3.id)
       AND NOT EXISTS (SELECT 1 FROM zdx_concern_patterns WHERE concern_id = c3.id))::int                  AS concerns_with_specs_no_pattern,
    (SELECT COUNT(*)
     FROM zdx_concern_specs cs
     JOIN zdx_concerns c4 ON c4.id = cs.concern_id
     WHERE c4.project_id = p.id AND lower(c4.name) = 'security')::int                                     AS security_concern_spec_count
FROM zdx_projects p
WHERE p.id = $1;
