-- name: CreateAtlasCoderef :one
INSERT INTO zdx_coderefs (project_id, file, start_line, end_line, symbol, sha)
VALUES (@project_id, @file, @start_line, @end_line, @symbol, @sha)
RETURNING *;

-- name: GetAtlasCoderef :one
SELECT * FROM zdx_coderefs WHERE project_id = @project_id AND id = @id;

-- name: ListAtlasCoderefs :many
SELECT * FROM zdx_coderefs WHERE project_id = @project_id ORDER BY id DESC;

-- name: ListAtlasCoderefsByFile :many
SELECT * FROM zdx_coderefs WHERE project_id = @project_id AND file = @file ORDER BY id DESC;

-- name: CreateAtlasNode :one
INSERT INTO zdx_nodes (project_id, kind, slug, title, description)
VALUES (@project_id, @kind, @slug, @title, @description)
RETURNING *;

-- name: GetAtlasNode :one
SELECT * FROM zdx_nodes WHERE project_id = @project_id AND kind = @kind AND slug = @slug;

-- name: GetAtlasNodeByID :one
SELECT * FROM zdx_nodes WHERE project_id = @project_id AND id = @id;

-- name: ListAtlasNodes :many
SELECT * FROM zdx_nodes
WHERE project_id = @project_id
  AND (@kind::text = '' OR kind = @kind)
ORDER BY kind, slug;

-- name: UpdateAtlasNode :exec
UPDATE zdx_nodes
SET title       = COALESCE(NULLIF(@title, ''), title),
    description = COALESCE(NULLIF(@description, ''), description),
    updated_at  = NOW()
WHERE project_id = @project_id AND id = @id;

-- name: DeleteAtlasNode :exec
DELETE FROM zdx_nodes WHERE project_id = @project_id AND id = @id;

-- name: CreateAtlasEdge :one
INSERT INTO zdx_edges (project_id, from_node_id, to_node_id, edge_type, label, weight)
VALUES (@project_id, @from_node_id, @to_node_id, @edge_type, @label, @weight)
RETURNING *;

-- name: ListAtlasEdgesFrom :many
SELECT e.*, n.kind AS to_kind, n.slug AS to_slug
FROM zdx_edges e
JOIN zdx_nodes n ON n.id = e.to_node_id
WHERE e.from_node_id = @from_node_id
ORDER BY e.id;

-- name: ListAtlasEdgesTo :many
SELECT * FROM zdx_edges WHERE to_node_id = @to_node_id ORDER BY id;

-- name: DeleteAtlasEdge :exec
DELETE FROM zdx_edges WHERE project_id = @project_id AND id = @id;

-- name: CreateAtlasChunk :one
INSERT INTO zdx_narrative_chunks (project_id, node_id, coderef_id, title, body, sha_at_write, verifier_kind)
VALUES (@project_id, @node_id, @coderef_id, @title, @body, @sha_at_write, @verifier_kind)
RETURNING *;

-- name: GetAtlasChunk :one
SELECT * FROM zdx_narrative_chunks WHERE project_id = @project_id AND id = @id;

-- name: ListAtlasChunks :many
SELECT * FROM zdx_narrative_chunks WHERE node_id = @node_id ORDER BY id;

-- name: UpdateAtlasChunk :exec
UPDATE zdx_narrative_chunks
SET title        = COALESCE(NULLIF(@title, ''), title),
    body         = COALESCE(NULLIF(@body, ''), body),
    sha_at_write = COALESCE(NULLIF(@sha_at_write, ''), sha_at_write),
    updated_at   = NOW()
WHERE project_id = @project_id AND id = @id;

-- name: DeleteAtlasChunk :exec
DELETE FROM zdx_narrative_chunks WHERE project_id = @project_id AND id = @id;
