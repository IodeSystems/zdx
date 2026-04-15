CREATE TABLE zdx_agents (
    id               TEXT        PRIMARY KEY,
    project_id       INT         NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    session_id       TEXT        NOT NULL DEFAULT '',
    worktree_path    TEXT        NOT NULL DEFAULT '',
    worktree_branch  TEXT        NOT NULL DEFAULT '',
    pid              INT         NOT NULL DEFAULT 0,
    status           TEXT        NOT NULL DEFAULT 'active',
    task_group       TEXT        NOT NULL DEFAULT '',
    compose_project  TEXT        NOT NULL DEFAULT '',
    server_port      INT         NOT NULL DEFAULT 0,
    database_url     TEXT        NOT NULL DEFAULT '',
    last_heartbeat   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE zdx_tasks
    ADD COLUMN claimed_by       TEXT        REFERENCES zdx_agents(id),
    ADD COLUMN claimed_at       TIMESTAMPTZ,
    ADD COLUMN lease_expires_at TIMESTAMPTZ;
