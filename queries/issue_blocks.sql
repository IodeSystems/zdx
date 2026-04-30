-- name: AddIssueBlock :exec
-- kind: 'sequencing' (default) for "X waits for Y" deps; 'composition' for tracker → child relationships.
-- ON CONFLICT updates kind so existing edges can be reclassified (e.g. backfilling pre-migration
-- tracker edges from 'sequencing' to 'composition').
INSERT INTO zdx_issue_blocks (issue_id, blocked_by_id, kind) VALUES ($1, $2, $3)
ON CONFLICT (issue_id, blocked_by_id) DO UPDATE SET kind = EXCLUDED.kind;

-- name: RemoveIssueBlock :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1 AND blocked_by_id = $2;

-- name: RemoveAllIssueBlocks :exec
DELETE FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockers :many
SELECT blocked_by_id FROM zdx_issue_blocks WHERE issue_id = $1;

-- name: ListIssueBlockersWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status, b.kind
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1;

-- name: ListIssueCompositionChildrenWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1 AND b.kind = 'composition';

-- name: ListIssueSequencingBlockersWithStatus :many
SELECT b.blocked_by_id AS id, COALESCE(i.status, 'open') AS status
FROM zdx_issue_blocks b
LEFT JOIN zdx_issues i ON i.id = b.blocked_by_id
WHERE b.issue_id = $1 AND b.kind = 'sequencing';

-- name: ListIssuesBlockedBy :many
SELECT issue_id FROM zdx_issue_blocks WHERE blocked_by_id = $1;

-- name: ListAncestorSequencingBlockers :many
-- For every issue in the project, walk up the composition chain (issue → parents)
-- and return any open sequencing blockers found anywhere in the ancestry, INCLUDING
-- the issue itself. Used by the queue's claim-check to gate composition children of
-- a blocked tracker without per-leaf wiring.
--
-- Composition edge layout (per --parent flow): (issue_id=parent, blocked_by_id=child).
-- So to walk from a child up, recurse: child's parent = (rows where blocked_by_id = child).id_field=issue_id.
WITH RECURSIVE ancestry(child_id, ancestor_id) AS (
  SELECT seed.id, seed.id FROM zdx_issues AS seed WHERE seed.project_id = $1
  UNION
  SELECT a.child_id, b.issue_id
  FROM ancestry a
  JOIN zdx_issue_blocks b ON b.blocked_by_id = a.ancestor_id
  WHERE b.kind = 'composition'
)
SELECT DISTINCT
  a.child_id      AS child_id,
  b.blocked_by_id AS blocker_id,
  a.ancestor_id   AS gated_ancestor
FROM ancestry a
JOIN zdx_issue_blocks b ON b.issue_id = a.ancestor_id
JOIN zdx_issues i        ON i.id = b.blocked_by_id
WHERE b.kind = 'sequencing'
  AND COALESCE(i.status, 'open') = 'open';
