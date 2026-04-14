-- name: ListProjectGoals :many
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM zdx_project_goals WHERE project_id = $1
ORDER BY priority, title;

-- name: GetProjectGoal :one
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM zdx_project_goals WHERE id = $1;

-- name: CreateProjectGoal :one
INSERT INTO zdx_project_goals (project_id, title, description, priority, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, title, description, priority, status, created_at, updated_at;

-- name: UpdateProjectGoal :exec
UPDATE zdx_project_goals
SET title       = $2,
    description = $3,
    priority    = $4,
    status      = $5,
    updated_at  = NOW()
WHERE id = $1;

-- name: DeleteProjectGoal :exec
DELETE FROM zdx_project_goals WHERE id = $1;

-- name: CountProjectGoals :one
SELECT count(*) FROM zdx_project_goals WHERE project_id = $1;

-- name: ListProjectConstraints :many
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM zdx_project_constraints WHERE project_id = $1
ORDER BY priority, title;

-- name: GetProjectConstraint :one
SELECT id, project_id, title, description, priority, status, created_at, updated_at
FROM zdx_project_constraints WHERE id = $1;

-- name: CreateProjectConstraint :one
INSERT INTO zdx_project_constraints (project_id, title, description, priority, status)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, project_id, title, description, priority, status, created_at, updated_at;

-- name: UpdateProjectConstraint :exec
UPDATE zdx_project_constraints
SET title       = $2,
    description = $3,
    priority    = $4,
    status      = $5,
    updated_at  = NOW()
WHERE id = $1;

-- name: DeleteProjectConstraint :exec
DELETE FROM zdx_project_constraints WHERE id = $1;

-- name: CountProjectConstraints :one
SELECT count(*) FROM zdx_project_constraints WHERE project_id = $1;
