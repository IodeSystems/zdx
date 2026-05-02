INSERT INTO zdx_patterns (project_id, name, description)
SELECT project_id, title, description
FROM zdx_project_constraints;

INSERT INTO zdx_concern_patterns (concern_id, pattern_id)
SELECT c.id, p.id
FROM zdx_patterns p
JOIN zdx_project_constraints pc ON pc.project_id = p.project_id AND pc.title = p.name
JOIN zdx_concerns c ON c.project_id = p.project_id
    AND c.name = CASE
        WHEN lower(pc.title || ' ' || pc.description) ~ 'ui|confirm|browser|dialog' THEN 'UX'
        WHEN lower(pc.title || ' ' || pc.description) ~ 'auth|token|permission|secret' THEN 'Security'
        WHEN lower(pc.title || ' ' || pc.description) ~ 'perf|slow|timeout|latency' THEN 'Latency'
        ELSE 'Functional'
    END
ON CONFLICT DO NOTHING;
