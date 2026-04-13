CREATE TABLE zdx_project_permissions (
    id         SERIAL PRIMARY KEY,
    user_id    INT  NOT NULL REFERENCES zdx_users(id) ON DELETE CASCADE,
    project_id INT  NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    role       TEXT NOT NULL DEFAULT 'member',  -- member | admin
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, project_id)
);

CREATE TABLE zdx_project_git_config (
    id         SERIAL PRIMARY KEY,
    project_id INT  NOT NULL UNIQUE REFERENCES zdx_projects(id) ON DELETE CASCADE,
    clone_url  TEXT NOT NULL DEFAULT '',
    auth_type  TEXT NOT NULL DEFAULT 'none',  -- none | token | ssh
    auth_token TEXT NOT NULL DEFAULT ''
);
