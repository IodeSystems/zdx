-- IS-1227 (TK-1813): zdx_env_agents tracks per-env daemon (dx-envd) liveness.
-- The daemon holds a persistent WS to dx-server and upserts a row on
-- registration + every heartbeat so `dx env list` and future deploy routing
-- (IS-1230) can see which environments have a live agent.
--
-- env_id + agent_id together identify a registration; UPSERT on (env_id,
-- agent_id) lets a daemon restart on the same host idempotently refresh its
-- metadata without leaking duplicate rows.

CREATE TABLE zdx_env_agents (
    id                  SERIAL PRIMARY KEY,
    env_id              INTEGER NOT NULL REFERENCES zdx_environments(id) ON DELETE CASCADE,
    agent_id            TEXT NOT NULL,
    hostname            TEXT NOT NULL DEFAULT '',
    version             TEXT NOT NULL DEFAULT '',
    os                  TEXT NOT NULL DEFAULT '',
    deployed_commit     TEXT NOT NULL DEFAULT '',
    uptime_secs         BIGINT NOT NULL DEFAULT 0,
    connected_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_heartbeat_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (env_id, agent_id)
);

CREATE INDEX zdx_env_agents_env_id_idx ON zdx_env_agents(env_id);
