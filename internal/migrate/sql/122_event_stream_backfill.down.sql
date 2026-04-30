-- Reverse backfill for IS-614.
-- NOTE: zdx_comment_reactions and zdx_comment_reads are recreated empty;
-- the original row data was dropped in the up migration and cannot be restored.

BEGIN;

-- Remove threads whose root event was a backfilled comment.
DELETE FROM zdx_event_threads
WHERE root_event_id IN (
    SELECT id FROM zdx_events
    WHERE event_type = 'comment' AND detail_json ? 'legacy_comment_id'
);

-- Remove backfilled comment events (identified by legacy_comment_id marker).
DELETE FROM zdx_events
WHERE event_type = 'comment' AND detail_json ? 'legacy_comment_id';

-- Remove backfilled revision events (identified by legacy_revision_id marker).
DELETE FROM zdx_events
WHERE event_type = 'revision' AND detail_json ? 'legacy_revision_id';

-- Recreate dropped tables (empty — original data is gone).
CREATE TABLE zdx_comment_reactions (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    comment_id INT  NOT NULL REFERENCES zdx_comments(id) ON DELETE CASCADE,
    emoji      TEXT NOT NULL,
    reactor    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (comment_id, emoji, reactor)
);
CREATE INDEX idx_comment_reactions_comment ON zdx_comment_reactions (comment_id);

CREATE TABLE zdx_comment_reads (
    id           SERIAL PRIMARY KEY,
    project_id   INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type  TEXT NOT NULL,
    target_id    TEXT NOT NULL,
    role         TEXT NOT NULL,
    last_read_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, target_type, target_id, role)
);

COMMIT;
