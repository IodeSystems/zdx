CREATE TABLE zdx_users (
    id            SERIAL PRIMARY KEY,
    email         TEXT UNIQUE NOT NULL,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'member',  -- member | admin
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_sessions (
    id         SERIAL PRIMARY KEY,
    user_id    INT  NOT NULL REFERENCES zdx_users(id) ON DELETE CASCADE,
    token      TEXT UNIQUE NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '30 days'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_api_keys (
    id           SERIAL PRIMARY KEY,
    user_id      INT  NOT NULL REFERENCES zdx_users(id) ON DELETE CASCADE,
    token        TEXT UNIQUE NOT NULL,
    name         TEXT NOT NULL,
    last_used_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_invites (
    id         SERIAL PRIMARY KEY,
    email      TEXT NOT NULL,
    token      TEXT UNIQUE NOT NULL,
    invited_by INT  NOT NULL REFERENCES zdx_users(id),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '7 days'),
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE zdx_oauth_identities (
    id         SERIAL PRIMARY KEY,
    user_id    INT  NOT NULL REFERENCES zdx_users(id) ON DELETE CASCADE,
    provider   TEXT NOT NULL,
    sub        TEXT NOT NULL,
    email      TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (provider, sub)
);

CREATE INDEX idx_oauth_identities_user ON zdx_oauth_identities (user_id);

CREATE TABLE zdx_oauth_states (
    state          TEXT PRIMARY KEY,
    provider       TEXT NOT NULL,
    code_verifier  TEXT NOT NULL DEFAULT '',
    redirect_to    TEXT NOT NULL DEFAULT '/',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at     TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '10 minutes')
);
