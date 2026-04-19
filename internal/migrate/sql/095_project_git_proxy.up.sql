ALTER TABLE zdx_projects
    ADD COLUMN upstream_url TEXT NOT NULL DEFAULT '',
    ADD COLUMN upstream_credentials TEXT NOT NULL DEFAULT '',
    ADD COLUMN git_enabled BOOLEAN NOT NULL DEFAULT FALSE;
