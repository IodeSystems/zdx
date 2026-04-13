-- name: ListFeatures :many
SELECT id, project_id, name, description, what, why, done_when, component, category
FROM zdx_features WHERE project_id = $1 ORDER BY category, name;

-- name: GetFeature :one
SELECT id, project_id, name, description, what, why, done_when, component, category
FROM zdx_features WHERE project_id = $1 AND name = $2;

-- name: UpsertFeature :one
INSERT INTO zdx_features (project_id, name, description)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
RETURNING id, project_id, name, description, what, why, done_when, component, category;

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
SELECT id, feature_id, description, kind FROM zdx_specs WHERE feature_id = $1 ORDER BY id;

-- name: ListSpecsForProject :many
SELECT s.id, s.feature_id, s.description, s.kind
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = $1
ORDER BY s.feature_id, s.id;

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
