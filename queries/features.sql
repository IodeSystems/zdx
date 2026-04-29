-- name: ListFeatures :many
SELECT id, project_id, name, description, what, why, done_when, component, category,
       kind, goal_id, parent_feature_id,
       metric_name, metric_unit, baseline_value, target_value, graph_url,
       last_reviewed_at
FROM zdx_features WHERE project_id = $1 ORDER BY category, name;

-- name: GetFeature :one
SELECT id, project_id, name, description, what, why, done_when, component, category,
       kind, goal_id, parent_feature_id,
       metric_name, metric_unit, baseline_value, target_value, graph_url,
       last_reviewed_at
FROM zdx_features WHERE project_id = $1 AND name = $2;

-- name: GetFeatureByID :one
SELECT id, project_id, name, description, what, why, done_when, component, category,
       kind, goal_id, parent_feature_id,
       metric_name, metric_unit, baseline_value, target_value, graph_url,
       last_reviewed_at
FROM zdx_features WHERE id = $1;

-- name: UpsertFeature :one
INSERT INTO zdx_features (project_id, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id, project_id, name, description, what, why, done_when, component, category,
          kind, goal_id, parent_feature_id,
          metric_name, metric_unit, baseline_value, target_value, graph_url,
          last_reviewed_at;

-- name: UpdateFeatureField :exec
UPDATE zdx_features
SET description = CASE WHEN @field::text = 'description' THEN @value::text ELSE description END,
    what        = CASE WHEN @field::text = 'what'        THEN @value::text ELSE what        END,
    why         = CASE WHEN @field::text = 'why'         THEN @value::text ELSE why         END,
    done_when   = CASE WHEN @field::text = 'done_when'   THEN @value::text ELSE done_when   END,
    component   = CASE WHEN @field::text = 'component'   THEN @value::text ELSE component   END,
    category    = CASE WHEN @field::text = 'category'    THEN @value::text ELSE category    END,
    kind        = CASE WHEN @field::text = 'kind'        THEN @value::text ELSE kind        END,
    metric_name = CASE WHEN @field::text = 'metric_name' THEN @value::text ELSE metric_name END,
    metric_unit = CASE WHEN @field::text = 'metric_unit' THEN @value::text ELSE metric_unit END,
    baseline_value = CASE WHEN @field::text = 'baseline_value' THEN @value::text ELSE baseline_value END,
    target_value   = CASE WHEN @field::text = 'target_value'   THEN @value::text ELSE target_value   END,
    graph_url      = CASE WHEN @field::text = 'graph_url'      THEN @value::text ELSE graph_url      END
WHERE project_id = @project_id AND name = @name;

-- name: UpdateFeatureGoal :exec
UPDATE zdx_features SET goal_id = @goal_id WHERE id = @id;

-- name: UpdateFeatureParent :exec
UPDATE zdx_features SET parent_feature_id = @parent_feature_id WHERE id = @id;

-- name: ClearFeatureParent :exec
UPDATE zdx_features SET parent_feature_id = NULL WHERE id = @id;

-- name: DeleteFeature :exec
DELETE FROM zdx_features WHERE id = $1;

-- name: ListChildFeatures :many
SELECT id, project_id, name, description, kind, goal_id, component
FROM zdx_features WHERE parent_feature_id = $1
ORDER BY name;

-- name: AddFeatureMultiplier :exec
INSERT INTO zdx_feature_multipliers (feature_id, multiplies_feature_id)
VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: RemoveFeatureMultiplier :exec
DELETE FROM zdx_feature_multipliers WHERE feature_id = $1 AND multiplies_feature_id = $2;

-- name: ListFeatureMultipliers :many
SELECT fm.multiplies_feature_id, f.name, f.description, f.kind
FROM zdx_feature_multipliers fm
JOIN zdx_features f ON f.id = fm.multiplies_feature_id
WHERE fm.feature_id = $1
ORDER BY f.name;

-- name: ListFeaturesByGoal :many
SELECT id, project_id, name, description, kind, component, parent_feature_id
FROM zdx_features WHERE goal_id = $1
ORDER BY name;

-- ── Specs ────────────────────────────────────────────────────────────────────

-- name: ListSpecs :many
SELECT id, feature_id, description, importance
FROM zdx_specs WHERE feature_id = $1 ORDER BY id;

-- name: ListSpecsForProject :many
SELECT s.id, s.feature_id, s.description, s.importance
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = $1
ORDER BY s.feature_id, s.id;

-- name: GetSpec :one
SELECT id, feature_id, description, importance
FROM zdx_specs WHERE id = $1;

-- name: GetSpecForProject :one
-- Fetch a spec by id, validating it belongs to the given project.
SELECT s.id, s.feature_id, s.description, s.importance
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE s.id = $1 AND f.project_id = $2;

-- name: ListUncoveredSpecs :many
-- Specs without linked tests AND without an open task in flight for them.
-- The "in-flight" check matches any open task (wip/ready/active) whose title
-- or text references the spec by id ("spec N" with word boundaries) — agents file
-- with varied titles or put the spec reference in the task body, so both fields
-- are checked to avoid re-emitting the nudge when work is already queued.
SELECT s.id, s.feature_id, s.description, s.importance, f.name AS feature_name
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
LEFT JOIN zdx_spec_tests st ON st.spec_id = s.id
WHERE f.project_id = $1
  AND st.spec_id IS NULL
  AND NOT EXISTS (
    SELECT 1 FROM zdx_tasks t
    WHERE t.project_id = $1
      AND t.status IN ('ready', 'wip', 'active')
      AND (
        t.spec = s.id::text
        OR t.title ~* ('\mspec\s+' || s.id::text || '\M')
        OR t.text ~* ('\mspec\s+' || s.id::text || '\M')
      )
  )
ORDER BY f.name, s.id;

-- name: ListTasksForSpec :many
-- Tasks (any status) whose title or text references spec N by ID ("spec N" word boundary).
SELECT t.id, t.title, t.status
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
JOIN zdx_tasks t ON t.project_id = f.project_id
WHERE s.id = $1
  AND (
    t.title ~* ('\mspec\s+' || s.id::text || '\M')
    OR t.text ~* ('\mspec\s+' || s.id::text || '\M')
  )
ORDER BY t.created_at;

-- name: UpdateSpecFeature :exec
UPDATE zdx_specs SET feature_id = @feature_id WHERE id = @id;

-- name: DeleteSpec :exec
DELETE FROM zdx_specs WHERE id = $1;

-- name: LinkSpecIssue :exec
INSERT INTO zdx_spec_issues (spec_id, issue_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnlinkSpecIssue :exec
DELETE FROM zdx_spec_issues WHERE spec_id = $1 AND issue_id = $2;

-- name: ListSpecIssues :many
SELECT si.spec_id, si.issue_id, i.title, i.status
FROM zdx_spec_issues si
JOIN zdx_issues i ON i.id = si.issue_id
WHERE si.spec_id = $1
ORDER BY si.created_at;

-- name: ListIssueSpecs :many
SELECT si.spec_id, si.issue_id, s.description, s.importance
FROM zdx_spec_issues si
JOIN zdx_specs s ON s.id = si.spec_id
WHERE si.issue_id = $1
ORDER BY si.spec_id;

-- name: MarkFeatureReviewed :exec
UPDATE zdx_features SET last_reviewed_at = NOW()
WHERE project_id = $1 AND name = $2;

-- name: ListStaleFeatures :many
SELECT id, project_id, name, description, what, why, done_when, component, category,
       kind, goal_id, parent_feature_id, last_reviewed_at
FROM zdx_features
WHERE project_id = @project_id
  AND (last_reviewed_at IS NULL OR last_reviewed_at < NOW() - (@stale_days::int || ' days')::interval)
ORDER BY last_reviewed_at NULLS FIRST, name;

-- name: AddSpec :one
INSERT INTO zdx_specs (feature_id, description, importance) VALUES ($1, $2, $3)
RETURNING id, feature_id, description, importance;

-- name: SearchFeatures :many
-- metaquery: off
SELECT id, name, description, category, kind
FROM zdx_features
WHERE project_id = @project_id
  AND (name ILIKE '%' || @query::text || '%' OR description ILIKE '%' || @query::text || '%')
ORDER BY name
LIMIT 10;
