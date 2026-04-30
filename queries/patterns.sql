-- name: InsertPattern :one
INSERT INTO zdx_patterns (project_id, name, description, code_refs)
VALUES ($1, $2, $3, $4)
RETURNING id, project_id, name, description, code_refs, created_at, updated_at;

-- name: UpdatePattern :one
UPDATE zdx_patterns
SET name        = $3,
    description = $4,
    code_refs   = $5,
    updated_at  = NOW()
WHERE project_id = $1 AND id = $2
RETURNING id, project_id, name, description, code_refs, created_at, updated_at;

-- name: GetPattern :one
SELECT id, project_id, name, description, code_refs, created_at, updated_at
FROM zdx_patterns
WHERE project_id = $1 AND id = $2;

-- name: DeletePattern :exec
DELETE FROM zdx_patterns
WHERE project_id = $1 AND id = $2;

-- name: ListPatterns :many
SELECT id, project_id, name, description, code_refs, created_at, updated_at
FROM zdx_patterns
WHERE project_id = $1
ORDER BY name;


-- name: SearchPatterns :many
-- metaquery: off
SELECT id, project_id, name, description, code_refs, created_at, updated_at
FROM zdx_patterns
WHERE project_id = $1
  AND (name ILIKE '%' || $2 || '%' OR description ILIKE '%' || $2 || '%')
ORDER BY name
LIMIT $3;

-- name: GetPatternsForConcern :many
SELECT id, project_id, name, description, code_refs, created_at, updated_at
FROM zdx_patterns
WHERE id IN (SELECT pattern_id FROM zdx_concern_patterns WHERE concern_id = $1)
ORDER BY id;
