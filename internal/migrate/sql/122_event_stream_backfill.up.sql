-- Backfill zdx_comments and zdx_proposal_versions into the unified event stream
-- (IS-614). Drops zdx_comment_reactions and zdx_comment_reads per IS-610.
-- zdx_comments and zdx_proposal_versions are left intact (dropped in IS-617).

BEGIN;

-- Temp mapping: legacy comment id → new event id (dropped at COMMIT).
CREATE TEMP TABLE _tmp_comment_event_map (
    legacy_id INT    NOT NULL,
    event_id  BIGINT NOT NULL
) ON COMMIT DROP;

-- 1. Backfill one event per comment; populate mapping for thread wiring.
WITH inserted AS (
    INSERT INTO zdx_events (
        project_id, target_type, target_id,
        event_type, author, author_kind,
        summary_json, detail_json,
        agent_process_result, created_at
    )
    SELECT
        c.project_id,
        c.target_type,
        c.target_id,
        'comment',
        c.author,
        CASE WHEN c.author_alias <> '' THEN 'agent' ELSE 'user' END,
        jsonb_build_object('body', c.body),
        jsonb_build_object('body', c.body, 'legacy_comment_id', c.id),
        NULL,
        c.created_at
    FROM zdx_comments c
    ORDER BY c.id
    RETURNING id, (detail_json->>'legacy_comment_id')::int AS legacy_id
)
INSERT INTO _tmp_comment_event_map (legacy_id, event_id)
SELECT legacy_id, id FROM inserted;

-- 2. Create one thread per root comment that has at least one reply.
--    root_event_id comes from the mapping populated above.
INSERT INTO zdx_event_threads (
    project_id, target_type, target_id, root_event_id, title, created_at
)
SELECT
    c.project_id, c.target_type, c.target_id,
    m.event_id,
    NULL,
    c.created_at
FROM zdx_comments c
JOIN _tmp_comment_event_map m ON m.legacy_id = c.id
WHERE c.parent_id IS NULL
  AND EXISTS (SELECT 1 FROM zdx_comments ch WHERE ch.parent_id = c.id);

-- 3. Walk every non-root comment to its root ancestor and set thread_id.
--    Recursion terminates when parent_id IS NULL (no join match).
WITH RECURSIVE path_to_root AS (
    SELECT id, parent_id, id AS leaf_id
    FROM zdx_comments
    WHERE parent_id IS NOT NULL
    UNION ALL
    SELECT c.id, c.parent_id, ptr.leaf_id
    FROM zdx_comments c
    JOIN path_to_root ptr ON c.id = ptr.parent_id
)
UPDATE zdx_events e
SET thread_id = t.id
FROM (
    SELECT DISTINCT id AS root_id, leaf_id
    FROM path_to_root
    WHERE parent_id IS NULL
) roots
JOIN _tmp_comment_event_map leaf_map ON leaf_map.legacy_id = roots.leaf_id
JOIN _tmp_comment_event_map root_map ON root_map.legacy_id = roots.root_id
JOIN zdx_event_threads t ON t.root_event_id = root_map.event_id
WHERE e.id = leaf_map.event_id;

-- 4. Backfill one revision event per zdx_proposal_version.
INSERT INTO zdx_events (
    project_id, target_type, target_id,
    event_type, author, author_kind,
    summary_json, detail_json,
    agent_process_result, created_at
)
SELECT
    p.project_id,
    'proposal',
    p.id::text,
    'revision',
    pv.edited_by,
    'user',
    jsonb_build_object('label', 'Version ' || pv.id),
    jsonb_build_object('body', pv.body, 'legacy_revision_id', pv.id, 'proposal_id', p.id),
    NULL,
    pv.edited_at
FROM zdx_proposal_versions pv
JOIN zdx_proposals p ON p.id = pv.proposal_id
ORDER BY pv.proposal_id, pv.edited_at;

-- 5. Drop legacy lookup tables (data unrecoverable after this point).
DROP TABLE IF EXISTS zdx_comment_reactions;
DROP TABLE IF EXISTS zdx_comment_reads;

COMMIT;
