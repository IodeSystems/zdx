-- Per-target comment-read high-water mark. Replaces the (dropped, IS-610)
-- zdx_comment_reads table with simpler semantics geared for the synthetic
-- read:comments todo:
--
--   - one row per (project, target_type, target_id)
--   - last_comment_id stores the highest comment id this target has been
--     marked seen up to (advanced when a read:comments todo resolves)
--   - the synthetic queue regenerator joins this table and only emits the
--     candidate when zdx_comments has rows newer than last_comment_id
--
-- Without this, ListTargetsWithComments returns every issue/feature with any
-- comment forever, the agent can't actually clear the todo, and the post-
-- resolve cycle detector fires on every iteration. Tracking the read state
-- removes the cycle at its source.

CREATE TABLE zdx_target_comments_seen (
    project_id      INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    target_type     TEXT NOT NULL,
    target_id       TEXT NOT NULL,
    last_comment_id INT  NOT NULL DEFAULT 0,
    seen_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (project_id, target_type, target_id)
);

-- Backfill: existing comments are considered already seen up to the current
-- max id for each target. Without this, the first post-deploy queue pass
-- would surface every commented target as "unread" and bury operators in
-- synthetic todos. The high-water mark advances naturally for new comments.
INSERT INTO zdx_target_comments_seen (project_id, target_type, target_id, last_comment_id, seen_at)
SELECT project_id, target_type, target_id, MAX(id), NOW()
FROM zdx_comments
GROUP BY project_id, target_type, target_id;
