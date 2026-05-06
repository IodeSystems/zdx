-- name: ListProjectGoals :many
SELECT id, project_id, title, description, priority, status, metric_name, metric_unit, created_at, updated_at
FROM zdx_project_goals WHERE project_id = $1
ORDER BY priority, title;

-- name: GetProjectGoal :one
SELECT id, project_id, title, description, priority, status, metric_name, metric_unit, created_at, updated_at
FROM zdx_project_goals WHERE id = $1;

-- name: CreateProjectGoal :one
INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id, project_id, title, description, priority, status, metric_name, metric_unit, created_at, updated_at;

-- name: UpdateProjectGoal :exec
UPDATE zdx_project_goals
SET title       = $2,
    description = $3,
    priority    = $4,
    status      = $5,
    metric_name = $6,
    metric_unit = $7,
    updated_at  = NOW()
WHERE id = $1;

-- name: DeleteProjectGoal :exec
DELETE FROM zdx_project_goals WHERE id = $1;

-- name: CountProjectGoals :one
SELECT count(*) FROM zdx_project_goals WHERE project_id = $1;

-- name: LinkGoalIssue :exec
INSERT INTO zdx_goal_issues (goal_id, issue_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnlinkGoalIssue :exec
DELETE FROM zdx_goal_issues WHERE goal_id = $1 AND issue_id = $2;

-- name: ListGoalIssues :many
SELECT issue_id FROM zdx_goal_issues WHERE goal_id = $1;

-- name: ListIssueGoals :many
SELECT g.id, g.project_id, g.title, g.description, g.priority, g.status, g.metric_name, g.metric_unit, g.created_at, g.updated_at
FROM zdx_project_goals g
JOIN zdx_goal_issues gi ON gi.goal_id = g.id
WHERE gi.issue_id = $1;
