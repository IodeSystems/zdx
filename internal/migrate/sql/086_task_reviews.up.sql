CREATE TABLE zdx_task_reviews (
    id              SERIAL PRIMARY KEY,
    project_id      INTEGER NOT NULL REFERENCES zdx_projects(id),
    task_id         TEXT    NOT NULL REFERENCES zdx_tasks(id) ON DELETE CASCADE,
    reviewer_role   TEXT    NOT NULL DEFAULT 'reviewer',
    reviewer_user_id INTEGER REFERENCES zdx_users(id),
    verdict         TEXT    NOT NULL DEFAULT '',
    body            TEXT    NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (project_id, task_id, reviewer_role)
);

CREATE INDEX zdx_task_reviews_task_idx ON zdx_task_reviews (project_id, task_id);

-- Best-effort backfill: convert pre-IS-252 task review comments
-- (body LIKE '## Review [...] ...') into review records.
INSERT INTO zdx_task_reviews (project_id, task_id, reviewer_role, verdict, body, created_at, updated_at)
SELECT
    c.project_id,
    c.target_id AS task_id,
    'reviewer' AS reviewer_role,
    COALESCE(NULLIF(SUBSTRING(c.body FROM '^## Review \[([^\]]+)\]'), ''), '') AS verdict,
    REGEXP_REPLACE(c.body, '^## Review \[[^\]]+\]\s*\n*', '') AS body,
    c.created_at,
    c.created_at AS updated_at
FROM zdx_comments c
JOIN zdx_tasks t ON t.id = c.target_id AND t.project_id = c.project_id
WHERE c.target_type = 'task'
  AND c.body LIKE '## Review [%'
  AND NOT EXISTS (
      SELECT 1 FROM zdx_task_reviews r
      WHERE r.project_id = c.project_id
        AND r.task_id = c.target_id
        AND r.reviewer_role = 'reviewer'
  );
