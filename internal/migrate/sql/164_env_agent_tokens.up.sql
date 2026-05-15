-- IS-1228 (TK-1816): zdx_env_agent_tokens stores per-env scoped tokens used by
-- dx-envd to authenticate to dx-server. Each row is a hashed bearer token
-- bound to a single env_id with a scope list (e.g. {"deploy:apply",
-- "schema:dump"}). Plaintext tokens are issued once at mint time and never
-- stored; lookups happen by token_hash (SHA-256 hex) on the middleware hot
-- path. Revocation is soft (revoked_at) so the audit trail is preserved.

CREATE TABLE zdx_env_agent_tokens (
    id           SERIAL PRIMARY KEY,
    env_id       INTEGER NOT NULL REFERENCES zdx_environments(id) ON DELETE CASCADE,
    token_hash   TEXT NOT NULL UNIQUE,
    scopes       TEXT[] NOT NULL DEFAULT '{}',
    name         TEXT NOT NULL DEFAULT 'env-agent',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX zdx_env_agent_tokens_env_id_idx
    ON zdx_env_agent_tokens (env_id)
    WHERE revoked_at IS NULL;
