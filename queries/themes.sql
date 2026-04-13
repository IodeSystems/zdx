-- name: ListThemes :many
SELECT t.id, t.name, t.description, t.priority, t.status, t.created_at,
       COALESCE(STRING_AGG(tb.issue_id, ','), '') AS blockers
FROM zdx_themes t
LEFT JOIN zdx_theme_blockers tb ON tb.theme_id = t.id
WHERE t.project_id = $1
GROUP BY t.id ORDER BY t.priority, t.name;

-- name: GetThemeByID :one
SELECT id, project_id, name, description, priority, status, created_at
FROM zdx_themes WHERE project_id = $1 AND id = $2;

-- name: GetThemeByName :one
SELECT id, project_id, name, description, priority, status, created_at
FROM zdx_themes WHERE project_id = $1 AND name = $2;

-- name: CreateTheme :one
INSERT INTO zdx_themes (project_id, name, description, priority)
VALUES (@project_id, @name, @description, @priority)
RETURNING id, project_id, name, description, priority, status, created_at;

-- name: UpdateThemeStatus :exec
UPDATE zdx_themes SET status = @status WHERE project_id = @project_id AND id = @id;

-- name: AddThemeBlocker :exec
INSERT INTO zdx_theme_blockers (theme_id, issue_id) VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: RemoveThemeBlocker :exec
DELETE FROM zdx_theme_blockers WHERE theme_id = $1 AND issue_id = $2;
