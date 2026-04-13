ALTER TABLE zdx_projects
    ADD COLUMN git_url    TEXT NOT NULL DEFAULT '',
    ADD COLUMN git_branch TEXT NOT NULL DEFAULT 'main',
    ADD COLUMN git_token  TEXT NOT NULL DEFAULT '';
