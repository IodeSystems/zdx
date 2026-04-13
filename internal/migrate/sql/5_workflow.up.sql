-- Structured work log: replaces / augments zdx_issue_work for journal-style entries.
CREATE TABLE zdx_work_log (
    id         SERIAL PRIMARY KEY,
    issue_id   TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    entry_type TEXT NOT NULL,   -- triage | tech | dev | note
    by_role    TEXT NOT NULL,   -- owner | tech | agent
    note       TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Owner + tech periodic self-assessments.
CREATE TABLE zdx_journal_entries (
    id             SERIAL PRIMARY KEY,
    project_id     INT  NOT NULL REFERENCES zdx_projects(id),
    role           TEXT NOT NULL,   -- owner | tech
    date           TEXT NOT NULL,   -- YYYY-MM-DD
    baseline       BOOLEAN NOT NULL DEFAULT false,
    tldr           TEXT NOT NULL DEFAULT '',
    assessment     TEXT NOT NULL DEFAULT '',
    concerns       TEXT NOT NULL DEFAULT '',  -- pipe-separated
    next           TEXT NOT NULL DEFAULT '',
    changelog_json TEXT NOT NULL DEFAULT '{}',
    state_json     TEXT NOT NULL DEFAULT '{}',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_journal_project_role ON zdx_journal_entries (project_id, role, date DESC);

-- Per-project sprint/review cadence tracking.
CREATE TABLE zdx_sprints (
    id                 SERIAL PRIMARY KEY,
    project_id         INT  NOT NULL UNIQUE REFERENCES zdx_projects(id),
    last_owner_review  TIMESTAMPTZ,
    last_tech_review   TIMESTAMPTZ,
    last_owner_journal TIMESTAMPTZ,
    last_tech_journal  TIMESTAMPTZ
);

-- Feature implementation plans (one per feature).
CREATE TABLE zdx_plans (
    id              SERIAL PRIMARY KEY,
    feature_id      INT  NOT NULL UNIQUE REFERENCES zdx_features(id) ON DELETE CASCADE,
    plan_type       TEXT NOT NULL DEFAULT 'implement',
    status          TEXT NOT NULL DEFAULT 'pending',  -- pending | approved | rejected
    complexity      TEXT NOT NULL DEFAULT '',
    approach        TEXT NOT NULL DEFAULT '',
    last_reviewed_at TIMESTAMPTZ
);

-- Themes: goal-oriented work streams grouping blocker issues.
CREATE TABLE zdx_themes (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id),
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority    INT  NOT NULL DEFAULT 2,
    status      TEXT NOT NULL DEFAULT 'active',  -- active | done | archived
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, name)
);

CREATE TABLE zdx_theme_blockers (
    theme_id INT  NOT NULL REFERENCES zdx_themes(id) ON DELETE CASCADE,
    issue_id TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    PRIMARY KEY (theme_id, issue_id)
);
