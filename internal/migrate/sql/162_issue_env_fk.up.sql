-- IS-1221 (TK-1801): make zdx_environments the single source of truth for an
-- issue's (trunk_branch, release_branch) pair. The 2026-05-14 babysit refusal
-- traced back to four overlapping branch sources drifting; this migration
-- adds the FK surface so the server-side resolver can collapse them to one.
--
-- zdx_issues.env_id is per-issue; zdx_projects.default_env_id is the
-- project fallback. Resolver order (issue.env_id, project.default_env_id);
-- 422 when neither resolves so we fail closed instead of silently picking
-- a wrong branch.
--
-- ON DELETE SET NULL on both columns so deleting an environment doesn't
-- cascade-orphan issues; the resolver will surface the unresolvable state.

ALTER TABLE zdx_issues
    ADD COLUMN env_id INTEGER REFERENCES zdx_environments(id) ON DELETE SET NULL;

CREATE INDEX zdx_issues_env_id_idx ON zdx_issues(env_id) WHERE env_id IS NOT NULL;

ALTER TABLE zdx_projects
    ADD COLUMN default_env_id INTEGER REFERENCES zdx_environments(id) ON DELETE SET NULL;
