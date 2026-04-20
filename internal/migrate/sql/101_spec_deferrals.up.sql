CREATE TABLE zdx_spec_deferrals (
    spec_id integer NOT NULL REFERENCES zdx_specs(id) ON DELETE CASCADE,
    issue_id text NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    note text NOT NULL DEFAULT '',
    created_at timestamp with time zone NOT NULL DEFAULT now(),
    PRIMARY KEY (spec_id, issue_id)
);

CREATE INDEX idx_spec_deferrals_issue ON zdx_spec_deferrals(issue_id);
