-- name: ListFeatures :many
SELECT id, project_id, name, description, what, why, done_when, component, category, last_reviewed_at
FROM zdx_features WHERE project_id = $1 ORDER BY category, name;

-- name: GetFeature :one
SELECT id, project_id, name, description, what, why, done_when, component, category, last_reviewed_at
FROM zdx_features WHERE project_id = $1 AND name = $2;

-- name: UpsertFeature :one
INSERT INTO zdx_features (project_id, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id, project_id, name, description, what, why, done_when, component, category, last_reviewed_at;

-- name: UpdateFeatureField :exec
UPDATE zdx_features
SET description = CASE WHEN @field::text = 'description' THEN @value::text ELSE description END,
    what        = CASE WHEN @field::text = 'what'        THEN @value::text ELSE what        END,
    why         = CASE WHEN @field::text = 'why'         THEN @value::text ELSE why         END,
    done_when   = CASE WHEN @field::text = 'done_when'   THEN @value::text ELSE done_when   END,
    component   = CASE WHEN @field::text = 'component'   THEN @value::text ELSE component   END,
    category    = CASE WHEN @field::text = 'category'    THEN @value::text ELSE category    END
WHERE project_id = @project_id AND name = @name;

-- name: DeleteFeature :exec
DELETE FROM zdx_features WHERE id = $1;

-- name: ListSpecs :many
SELECT id, feature_id, description, kind, deferred, deferred_reason FROM zdx_specs WHERE feature_id = $1 ORDER BY id;

-- name: ListSpecsForProject :many
SELECT s.id, s.feature_id, s.description, s.kind, s.deferred, s.deferred_reason
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = $1
ORDER BY s.feature_id, s.id;

-- name: GetSpec :one
SELECT id, feature_id, description, kind, deferred, deferred_reason FROM zdx_specs WHERE id = $1;

-- name: ListUncoveredSpecs :many
-- Specs that have no entries in zdx_spec_tests (no test coverage) and are not deferred.
SELECT s.id, s.feature_id, s.description, s.kind, f.name AS feature_name
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
LEFT JOIN zdx_spec_tests st ON st.spec_id = s.id
WHERE f.project_id = $1
  AND st.spec_id IS NULL
  AND s.deferred = false
ORDER BY f.name, s.id;

-- name: DeferSpec :exec
UPDATE zdx_specs SET deferred = true, deferred_reason = @reason WHERE id = @id;

-- name: UndeferSpec :exec
UPDATE zdx_specs SET deferred = false, deferred_reason = '' WHERE id = $1;

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
SELECT si.spec_id, si.issue_id, s.description, s.kind, s.deferred
FROM zdx_spec_issues si
JOIN zdx_specs s ON s.id = si.spec_id
WHERE si.issue_id = $1
ORDER BY si.spec_id;

-- name: MarkFeatureReviewed :exec
UPDATE zdx_features SET last_reviewed_at = NOW()
WHERE project_id = $1 AND name = $2;

-- name: ListStaleFeatures :many
-- Features not reviewed in more than @stale_days days (or never reviewed).
SELECT id, project_id, name, description, what, why, done_when, component, category, last_reviewed_at
FROM zdx_features
WHERE project_id = @project_id
  AND (last_reviewed_at IS NULL OR last_reviewed_at < NOW() - (@stale_days::int || ' days')::interval)
ORDER BY last_reviewed_at NULLS FIRST, name;

-- name: AddSpec :one
INSERT INTO zdx_specs (feature_id, description, kind) VALUES ($1, $2, $3)
RETURNING id, feature_id, description, kind;

-- name: UpsertPlan :one
INSERT INTO zdx_plans (feature_id, plan_type, complexity, approach, status)
VALUES (@feature_id, @plan_type, @complexity, @approach, 'pending')
ON CONFLICT (feature_id) DO UPDATE
SET plan_type = EXCLUDED.plan_type,
    complexity = EXCLUDED.complexity,
    approach = EXCLUDED.approach,
    last_reviewed_at = NOW()
RETURNING id, feature_id, plan_type, status, complexity, approach, last_reviewed_at;

-- name: GetPlanByFeature :one
SELECT id, feature_id, plan_type, status, complexity, approach, last_reviewed_at
FROM zdx_plans WHERE feature_id = $1;
