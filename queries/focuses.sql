-- name: ListFocuses :many
-- metaquery: off
SELECT f.id, f.name, f.description, f.priority, f.status, f.created_at,
       f.started_at, f.ended_at,
       COALESCE(
         JSON_AGG(
           JSON_BUILD_OBJECT(
             'id', fb.issue_id,
             'status', COALESCE(i.status, 'open'),
             'justification', fb.justification
           )
         ) FILTER (WHERE fb.issue_id IS NOT NULL),
         '[]'::json
       ) AS attributions
FROM zdx_focuses f
LEFT JOIN zdx_focus_blockers fb ON fb.focus_id = f.id
LEFT JOIN zdx_issues i ON i.id = fb.issue_id
WHERE f.project_id = $1
GROUP BY f.id ORDER BY f.priority, f.name;

-- name: GetFocusByID :one
SELECT id, project_id, name, description, priority, status, created_at, started_at, ended_at
FROM zdx_focuses WHERE project_id = $1 AND id = $2;

-- name: GetFocusByName :one
SELECT id, project_id, name, description, priority, status, created_at, started_at, ended_at
FROM zdx_focuses WHERE project_id = $1 AND name = $2;

-- name: CreateFocus :one
INSERT INTO zdx_focuses (project_id, name, description, priority)
VALUES (@project_id, @name, @description, @priority)
RETURNING id, project_id, name, description, priority, status, created_at, started_at, ended_at;

-- name: UpdateFocusStatus :exec
UPDATE zdx_focuses SET status = @status WHERE project_id = @project_id AND id = @id;

-- name: UpdateFocusDates :exec
UPDATE zdx_focuses SET started_at = @started_at, ended_at = @ended_at
WHERE project_id = @project_id AND id = @id;

-- name: AddFocusBlocker :exec
INSERT INTO zdx_focus_blockers (focus_id, issue_id, justification) VALUES ($1, $2, $3)
ON CONFLICT (focus_id, issue_id) DO UPDATE SET justification = EXCLUDED.justification;

-- name: RemoveFocusBlocker :exec
DELETE FROM zdx_focus_blockers WHERE focus_id = $1 AND issue_id = $2;

-- name: AddFocusFeature :exec
INSERT INTO zdx_focus_features (focus_id, feature_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveFocusFeature :exec
DELETE FROM zdx_focus_features WHERE focus_id = $1 AND feature_id = $2;

-- name: ListFocusFeatures :many
SELECT ff.feature_id, f.name, f.description, f.kind, f.goal_id, f.component
FROM zdx_focus_features ff
JOIN zdx_features f ON f.id = ff.feature_id
WHERE ff.focus_id = $1
ORDER BY f.name;

-- name: ListFeatureFocuses :many
SELECT fo.id, fo.name, fo.priority, fo.status
FROM zdx_focus_features ff
JOIN zdx_focuses fo ON fo.id = ff.focus_id
WHERE ff.feature_id = $1
ORDER BY fo.priority, fo.name;

-- name: SearchFocuses :many
-- metaquery: off
SELECT id, name, description, status
FROM zdx_focuses
WHERE project_id = @project_id
  AND (name ILIKE '%' || @query::text || '%' OR description ILIKE '%' || @query::text || '%')
ORDER BY priority, name
LIMIT 10;
