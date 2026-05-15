-- IS-1230 (TK-1825): zdx_deploy_requests is the requester→consumer bridge in
-- the env-agent deploy flow. A slot agent (worker scope or higher) inserts a
-- pending row via POST /api/dx/envs/{slug}/deploy-requests; the env-agent
-- (dx-envd, env-scoped token) picks it up over its persistent WS, applies the
-- signed package, and posts the deploy-record back. The row stays as the
-- audit trail of who asked for what when.
--
-- status starts at 'pending'. Future tickets (IS-1229, IS-1231) advance it
-- through 'accepted' → 'in_progress' → 'succeeded'/'failed' as dx-envd reports
-- back. blocking_issue_id is the optional reverse ref to a tracker issue that
-- the deploy is meant to unblock (e.g. "ship hotfix for IS-1234").

CREATE TABLE zdx_deploy_requests (
    id                     SERIAL PRIMARY KEY,
    env_id                 INTEGER NOT NULL REFERENCES zdx_environments(id) ON DELETE CASCADE,
    commit_sha             TEXT NOT NULL,
    requested_by_user_id   INTEGER REFERENCES zdx_users(id) ON DELETE SET NULL,
    reason                 TEXT NOT NULL DEFAULT '',
    blocking_issue_id      TEXT REFERENCES zdx_issues(id) ON DELETE SET NULL,
    status                 TEXT NOT NULL DEFAULT 'pending',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX zdx_deploy_requests_env_status_idx
    ON zdx_deploy_requests (env_id, status, created_at DESC);
