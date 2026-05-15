-- IS-1230 (TK-1824/TK-1825): zdx_deploy_requests is the requester→consumer bridge
-- in the env-agent deploy flow. A slot agent (worker scope or higher) inserts
-- a pending row via POST /api/dx/envs/{slug}/deploy-requests; the env-agent
-- (dx-envd, env-scoped token) picks it up over its persistent WS, applies the
-- signed package, and posts the deploy-record back. The row stays as the
-- audit trail of who asked for what when.
--
-- Lifecycle: pending → accepted (dx-envd has claimed it) → succeeded | failed.
-- accepted_at / completed_at are set by the Mark* queries as the request moves
-- through dx-envd's apply loop. failure_text captures the terminal error when
-- status='failed' (empty otherwise).
--
-- blocking_issue_id is the optional reverse ref to a tracker issue that the
-- deploy is meant to unblock (e.g. "ship hotfix for IS-1234"). It's TEXT
-- because zdx_issues.id is the human-readable IS-N key.
--
-- requested_by_token_id is set for slot-agent tokens once IS-1228's middleware
-- plumbs the token id into ctx; nullable so the requester can also be a
-- human-API-key call where only the user_id is meaningful.

CREATE TABLE zdx_deploy_requests (
    id                     SERIAL PRIMARY KEY,
    env_id                 INTEGER NOT NULL REFERENCES zdx_environments(id) ON DELETE CASCADE,
    commit_sha             TEXT NOT NULL,
    requested_by_user_id   INTEGER REFERENCES zdx_users(id) ON DELETE SET NULL,
    requested_by_token_id  INTEGER REFERENCES zdx_env_agent_tokens(id) ON DELETE SET NULL,
    reason                 TEXT NOT NULL DEFAULT '',
    blocking_issue_id      TEXT REFERENCES zdx_issues(id) ON DELETE SET NULL,
    status                 TEXT NOT NULL DEFAULT 'pending',
    accepted_at            TIMESTAMPTZ,
    completed_at           TIMESTAMPTZ,
    failure_text           TEXT NOT NULL DEFAULT '',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Pending-lookup index for the dx-envd poller. (env_id, status, created_at)
-- lets the consumer cheaply find its oldest pending row.
CREATE INDEX zdx_deploy_requests_env_status_idx
    ON zdx_deploy_requests (env_id, status, created_at);
