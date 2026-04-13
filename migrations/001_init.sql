CREATE TABLE zdx_projects (
    id         SERIAL PRIMARY KEY,
    slug       TEXT NOT NULL UNIQUE,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_issues (
    id         TEXT PRIMARY KEY,        -- IS-N, generated server-side
    project_id INT  NOT NULL REFERENCES zdx_projects(id),
    title      TEXT NOT NULL,
    status     TEXT NOT NULL DEFAULT 'open',   -- open | closed
    priority   TEXT NOT NULL DEFAULT '',       -- 1-5
    component  TEXT NOT NULL DEFAULT '',
    context    TEXT NOT NULL DEFAULT '',
    blocked_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_issue_work (
    id         SERIAL PRIMARY KEY,
    issue_id   TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    agent      TEXT NOT NULL,
    note       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_tasks (
    id           TEXT PRIMARY KEY,     -- TK-N, generated server-side
    project_id   INT  NOT NULL REFERENCES zdx_projects(id),
    text         TEXT NOT NULL,
    feature      TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending',  -- pending | in_progress | done | blocked
    reason       TEXT NOT NULL DEFAULT '',
    issue        TEXT NOT NULL DEFAULT '',
    depends      TEXT NOT NULL DEFAULT '',         -- comma-separated task IDs
    test_plan    TEXT NOT NULL DEFAULT '',
    test_refs    TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE zdx_features (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    what        TEXT NOT NULL DEFAULT '',
    why         TEXT NOT NULL DEFAULT '',
    done_when   TEXT NOT NULL DEFAULT '',
    component   TEXT NOT NULL DEFAULT '',
    UNIQUE (project_id, name)
);

CREATE TABLE zdx_specs (
    id          SERIAL PRIMARY KEY,
    feature_id  INT  NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    kind        TEXT NOT NULL DEFAULT 'must'   -- must | should | could
);

-- Per-project counter table for generating IS-N / TK-N IDs atomically.
CREATE TABLE zdx_id_seq (
    project_id INT  NOT NULL REFERENCES zdx_projects(id),
    kind       TEXT NOT NULL,   -- 'issue' | 'task'
    next_val   INT  NOT NULL DEFAULT 1,
    PRIMARY KEY (project_id, kind)
);
