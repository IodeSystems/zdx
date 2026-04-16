-- name: CreatePlan :one
INSERT INTO zdx_plans (project_id, title, body, plan_type, complexity, approach, feature_id, focus_id, issue_id)
VALUES (@project_id, @title, @body, @plan_type, @complexity, @approach, @feature_id, @focus_id, @issue_id)
RETURNING id, project_id, title, body, plan_type, status, complexity, approach,
          feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at;

-- name: GetPlan :one
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE id = $1;

-- name: GetPlanByFeature :one
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE feature_id = $1 LIMIT 1;

-- name: ListPlans :many
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE project_id = $1
ORDER BY updated_at DESC;

-- name: ListPlansByFocus :many
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE focus_id = $1
ORDER BY updated_at DESC;

-- name: ListPlansByFeature :many
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE feature_id = $1
ORDER BY updated_at DESC;

-- name: ListPlansByIssue :many
SELECT id, project_id, title, body, plan_type, status, complexity, approach,
       feature_id, focus_id, issue_id, last_reviewed_at, created_at, updated_at
FROM zdx_plans WHERE issue_id = $1
ORDER BY updated_at DESC;

-- name: UpdatePlan :exec
UPDATE zdx_plans
SET title     = @title,
    body      = @body,
    plan_type = @plan_type,
    status    = @status,
    complexity = @complexity,
    approach  = @approach,
    updated_at = NOW()
WHERE id = @id;

-- name: DeletePlan :exec
DELETE FROM zdx_plans WHERE id = $1;

-- ── Plan steps ───────────────────────────────────────────────────────────────

-- name: CreatePlanStep :one
INSERT INTO zdx_plan_steps (plan_id, seq, text, status, depends_on)
VALUES (@plan_id, @seq, @text, 'pending', @depends_on)
RETURNING id, plan_id, seq, text, status, depends_on, created_at, updated_at;

-- name: ListPlanSteps :many
SELECT id, plan_id, seq, text, status, depends_on, created_at, updated_at
FROM zdx_plan_steps WHERE plan_id = $1
ORDER BY seq;

-- name: GetPlanStep :one
SELECT id, plan_id, seq, text, status, depends_on, created_at, updated_at
FROM zdx_plan_steps WHERE id = $1;

-- name: UpdatePlanStep :exec
UPDATE zdx_plan_steps
SET text       = @text,
    status     = @status,
    depends_on = @depends_on,
    updated_at = NOW()
WHERE id = @id;

-- name: UpdatePlanStepSeq :exec
UPDATE zdx_plan_steps SET seq = @seq, updated_at = NOW() WHERE id = @id;

-- name: DeletePlanStep :exec
DELETE FROM zdx_plan_steps WHERE id = $1;

-- ── Plan step refs (discovery spawns) ────────────────────────────────────────

-- name: CreatePlanStepRef :exec
INSERT INTO zdx_plan_step_refs (step_id, target_type, target_id)
VALUES ($1, $2, $3) ON CONFLICT DO NOTHING;

-- name: DeletePlanStepRef :exec
DELETE FROM zdx_plan_step_refs WHERE step_id = $1 AND target_type = $2 AND target_id = $3;

-- name: ListPlanStepRefs :many
SELECT step_id, target_type, target_id
FROM zdx_plan_step_refs WHERE step_id = $1
ORDER BY target_type, target_id;
