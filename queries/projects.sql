-- name: ListProjects :many
SELECT id, slug, name, created_at, git_url, git_branch, git_token FROM zdx_projects ORDER BY name;

-- name: GetProjectBySlug :one
SELECT id, slug, name, created_at, git_url, git_branch, git_token FROM zdx_projects WHERE slug = $1;

-- name: CreateProject :one
INSERT INTO zdx_projects (slug, name) VALUES ($1, $2)
RETURNING id, slug, name, created_at;

-- name: GetProjectGitConfig :one
SELECT slug, git_url, git_branch, git_token FROM zdx_projects WHERE slug = $1;

-- name: SetProjectGitConfig :exec
UPDATE zdx_projects SET git_url = @git_url, git_branch = @git_branch, git_token = @git_token WHERE slug = @slug;

-- name: NextID :one
INSERT INTO zdx_id_seq (kind, next_val) VALUES ($1, 2)
ON CONFLICT (kind) DO UPDATE SET next_val = zdx_id_seq.next_val + 1
RETURNING next_val - 1 AS val;
