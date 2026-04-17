-- Project classification drives the maturity vine (which rungs/checks apply).
ALTER TABLE zdx_projects ADD COLUMN classification text NOT NULL DEFAULT '';

-- Doctor deferrals: user deferred a recommendation, don't nag until next rung.
CREATE TABLE zdx_doctor_deferrals (
    id serial PRIMARY KEY,
    project_id integer NOT NULL REFERENCES zdx_projects(id),
    check_name text NOT NULL,
    rung text NOT NULL DEFAULT '',
    deferred_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (project_id, check_name)
);
