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

-- name: ListStaleChunksByProject :many
SELECT c.*, n.kind AS node_kind, n.slug AS node_slug,
       COALESCE(cr.file, '') AS coderef_file
FROM zdx_narrative_chunks c
JOIN zdx_nodes n ON n.id = c.node_id
LEFT JOIN zdx_coderefs cr ON cr.id = c.coderef_id
WHERE c.project_id = @project_id
  AND c.status = 'stale'
ORDER BY c.id;

-- name: MarkChunksStaleByFiles :exec
UPDATE zdx_narrative_chunks c
SET status     = 'stale',
    updated_at = NOW()
WHERE c.project_id = @project_id
  AND c.status = 'fresh'
  AND c.coderef_id IN (
    SELECT cr.id FROM zdx_coderefs cr
    WHERE cr.project_id = @project_id
      AND cr.file = ANY(@files::text[])
      AND cr.sha != @current_sha
  );

-- name: VerifyChunkStillAccurate :one
UPDATE zdx_narrative_chunks
SET status      = 'fresh',
    verified_at = NOW(),
    verified_by = @verified_by,
    sha_at_write = CASE WHEN @sha::text <> '' THEN @sha ELSE sha_at_write END,
    updated_at  = NOW()
WHERE project_id = @project_id AND id = @id
RETURNING *;

-- name: UpdateChunkBodyAndVerify :one
UPDATE zdx_narrative_chunks
SET body        = @body,
    status      = 'fresh',
    verified_at = NOW(),
    verified_by = @verified_by,
    sha_at_write = CASE WHEN @sha::text <> '' THEN @sha ELSE sha_at_write END,
    updated_at  = NOW()
WHERE project_id = @project_id AND id = @id
RETURNING *;

-- name: MarkChunkBroken :one
UPDATE zdx_narrative_chunks
SET status      = 'broken',
    verified_at = NOW(),
    verified_by = @verified_by,
    updated_at  = NOW()
WHERE project_id = @project_id AND id = @id
RETURNING *;

-- name: GetAtlasNodeSubgraph :many
-- Recursive BFS outward from @root_node_id following zdx_edges.from_node_id.
-- Caps traversal at @max_depth (callers should clamp to <= 2). Cycles are
-- prevented by tracking the visited path as an array. The root is excluded
-- from the result set; each remaining node is returned once at its shortest
-- depth from the root.
WITH RECURSIVE bfs AS (
    SELECT
        @root_node_id::bigint AS node_id,
        0::int                AS depth,
        ''::text              AS via_edge_type,
        0::bigint             AS via_from_node_id,
        ARRAY[@root_node_id::bigint] AS visited
    UNION ALL
    SELECT
        e.to_node_id,
        b.depth + 1,
        e.edge_type,
        e.from_node_id,
        b.visited || e.to_node_id
    FROM bfs b
    JOIN zdx_edges e ON e.from_node_id = b.node_id
    WHERE b.depth < @max_depth::int
      AND NOT (e.to_node_id = ANY(b.visited))
)
SELECT DISTINCT ON (n.id)
    n.id,
    n.kind,
    n.slug,
    n.title,
    n.description,
    b.depth::int           AS depth,
    b.via_edge_type::text  AS via_edge_type,
    b.via_from_node_id     AS via_from_node_id
FROM bfs b
JOIN zdx_nodes n ON n.id = b.node_id
WHERE n.project_id = @project_id
  AND b.depth > 0
ORDER BY n.id, b.depth ASC, b.via_edge_type ASC;

-- name: GetAtlasNodeStats :one
-- Returns drift and demo-coverage stats for a single node.
--   stale_count       — chunks whose coderef.sha has drifted from sha_at_write
--   total_with_coderef — chunks anchored to a coderef with a recorded sha_at_write
--   demo_coverage_count — outbound edges of type 'verified_by'
WITH chunks AS (
    SELECT c.coderef_id, c.sha_at_write, cr.sha AS current_sha
    FROM zdx_narrative_chunks c
    LEFT JOIN zdx_coderefs cr ON cr.id = c.coderef_id
    WHERE c.node_id = @node_id
)
SELECT
    COALESCE((
        SELECT COUNT(*) FROM chunks
        WHERE coderef_id IS NOT NULL
          AND sha_at_write != ''
          AND sha_at_write != current_sha
    ), 0)::bigint AS stale_count,
    COALESCE((
        SELECT COUNT(*) FROM chunks
        WHERE coderef_id IS NOT NULL
          AND sha_at_write != ''
    ), 0)::bigint AS total_with_coderef,
    COALESCE((
        SELECT COUNT(*) FROM zdx_edges
        WHERE from_node_id = @node_id
          AND edge_type = 'verified_by'
    ), 0)::bigint AS demo_coverage_count;
