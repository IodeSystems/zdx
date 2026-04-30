CREATE TABLE zdx_concerns (
    id         SERIAL PRIMARY KEY,
    project_id integer NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    name       text NOT NULL,
    description text NOT NULL DEFAULT '',
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE(project_id, name)
);

CREATE TABLE zdx_concern_features (
    concern_id integer NOT NULL REFERENCES zdx_concerns(id) ON DELETE CASCADE,
    feature_id integer NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    PRIMARY KEY (concern_id, feature_id)
);

CREATE TABLE zdx_concern_specs (
    concern_id integer NOT NULL REFERENCES zdx_concerns(id) ON DELETE CASCADE,
    spec_id    integer NOT NULL REFERENCES zdx_specs(id) ON DELETE CASCADE,
    PRIMARY KEY (concern_id, spec_id)
);

CREATE TABLE zdx_concern_patterns (
    concern_id integer NOT NULL REFERENCES zdx_concerns(id) ON DELETE CASCADE,
    pattern_id integer NOT NULL REFERENCES zdx_patterns(id) ON DELETE CASCADE,
    PRIMARY KEY (concern_id, pattern_id)
);

CREATE TABLE zdx_concern_issues (
    concern_id integer NOT NULL REFERENCES zdx_concerns(id) ON DELETE CASCADE,
    issue_id   text    NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    PRIMARY KEY (concern_id, issue_id)
);

INSERT INTO zdx_concerns (project_id, name, description)
SELECT p.id, c.name, c.description
FROM zdx_projects p
CROSS JOIN (VALUES
    ('UX', 'User experience: clarity, friction, affordances'),
    ('Security', 'Authentication, authorization, data protection'),
    ('Latency', 'Response time, throughput, perceived performance'),
    ('Compatibility', 'Cross-platform, cross-version, backwards compat'),
    ('Operability', 'Observability, deployability, supportability'),
    ('Accessibility', 'WCAG, keyboard nav, screen reader support'),
    ('Functional', 'Correctness of business logic / behavior')
) AS c(name, description);
