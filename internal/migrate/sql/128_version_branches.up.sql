CREATE TABLE zdx_version_branches (
    id         bigserial   NOT NULL PRIMARY KEY,
    project_id integer     NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name       text        NOT NULL,
    type       text        NOT NULL CHECK (type IN ('dev', 'named')),
    semver     text,
    status     text        NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'eol')),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, name)
);
