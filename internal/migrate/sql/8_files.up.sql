CREATE TABLE zdx_files (
    id         SERIAL PRIMARY KEY,
    provider   TEXT   NOT NULL,   -- fs | s3
    path       TEXT   NOT NULL,   -- relative to provider root
    mime_type  TEXT   NOT NULL DEFAULT '',
    size_bytes BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_issue_files (
    id         SERIAL PRIMARY KEY,
    issue_id   TEXT NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    file_id    INT  NOT NULL REFERENCES zdx_files(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL DEFAULT 'attachment',  -- screenshot | attachment
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (issue_id, file_id)
);

CREATE INDEX idx_issue_files_issue ON zdx_issue_files (issue_id);
