CREATE TABLE zdx_project_goals (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority    INT  NOT NULL DEFAULT 1,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, title)
);
CREATE INDEX idx_project_goals_project ON zdx_project_goals (project_id);

CREATE TABLE zdx_project_constraints (
    id          SERIAL PRIMARY KEY,
    project_id  INT  NOT NULL REFERENCES zdx_projects(id),
    title       TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    priority    INT  NOT NULL DEFAULT 1,
    status      TEXT NOT NULL DEFAULT 'active',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (project_id, title)
);
CREATE INDEX idx_project_constraints_project ON zdx_project_constraints (project_id);
