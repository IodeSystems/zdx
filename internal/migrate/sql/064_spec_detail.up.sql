ALTER TABLE zdx_specs ADD COLUMN deferred_reason text NOT NULL DEFAULT '';

CREATE TABLE zdx_spec_issues (
    spec_id integer NOT NULL REFERENCES zdx_specs(id) ON DELETE CASCADE,
    issue_id text NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (spec_id, issue_id)
);

CREATE INDEX idx_spec_issues_issue ON zdx_spec_issues(issue_id);
