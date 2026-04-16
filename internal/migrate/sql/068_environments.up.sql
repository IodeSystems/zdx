CREATE TABLE zdx_environments (
    id                   SERIAL PRIMARY KEY,
    project_id           INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name                 TEXT NOT NULL,
    url                  TEXT NOT NULL DEFAULT '',
    current_build_sha    TEXT NOT NULL DEFAULT '',
    current_build_branch TEXT NOT NULL DEFAULT '',
    deployed_at          TIMESTAMPTZ,
    deployed_by_user_id  INT REFERENCES zdx_users(id) ON DELETE SET NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX zdx_environments_project_name ON zdx_environments (project_id, name);

CREATE TABLE zdx_deploys (
    id                  SERIAL PRIMARY KEY,
    environment_id      INT NOT NULL REFERENCES zdx_environments(id) ON DELETE CASCADE,
    build_sha           TEXT NOT NULL,
    build_branch        TEXT NOT NULL DEFAULT '',
    deployed_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deployed_by_user_id INT REFERENCES zdx_users(id) ON DELETE SET NULL,
    status              TEXT NOT NULL DEFAULT 'success'
);
CREATE INDEX zdx_deploys_env_time ON zdx_deploys (environment_id, deployed_at DESC);
