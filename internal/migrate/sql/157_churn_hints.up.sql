-- TK-1789: churn-hints primitive for Stream 5a (IS-1052).
--
-- A churn hint is an observed concentration of unfocused activity (rebuilds,
-- compactions, contradictory verdicts, repeated reviews, etc.) that may, but
-- does not necessarily, deserve to be promoted into a tracked issue or
-- pattern. Hints accumulate refs (the underlying observations) and can carry
-- one or more resolutions (an issue and/or a pattern they were closed into).
--
-- Embeddings for hints live in per-project .zvec files via internal/server/
-- embedder.go (IS-1156 will extend the Embedder interface) — intentionally
-- not a column here.

CREATE TABLE zdx_churn_hints (
    id              BIGSERIAL PRIMARY KEY,
    project_id      INT NOT NULL REFERENCES zdx_projects(id) ON DELETE CASCADE,
    title           TEXT NOT NULL,
    summary         TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'open'
                    CHECK (status IN ('open', 'stale-closed', 'resolved-closed')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_active_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    closed_at       TIMESTAMPTZ,
    closed_by_ref   TEXT
);

CREATE INDEX idx_churn_hints_project_status_active
    ON zdx_churn_hints (project_id, status, last_active_at DESC);

CREATE TABLE zdx_churn_hint_refs (
    id          SERIAL PRIMARY KEY,
    hint_id     BIGINT NOT NULL REFERENCES zdx_churn_hints(id) ON DELETE CASCADE,
    ref_type    TEXT NOT NULL
                CHECK (ref_type IN ('issue', 'task', 'session', 'file', 'review', 'verdict', 'operator')),
    ref_id      TEXT NOT NULL,
    observation TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_churn_hint_refs_hint ON zdx_churn_hint_refs (hint_id);

-- One hint can carry both an 'issue' and a 'pattern' resolution; no uniqueness
-- constraint on hint_id.
CREATE TABLE zdx_churn_hint_resolutions (
    id              SERIAL PRIMARY KEY,
    hint_id         BIGINT NOT NULL REFERENCES zdx_churn_hints(id) ON DELETE CASCADE,
    resolution_type TEXT NOT NULL CHECK (resolution_type IN ('issue', 'pattern')),
    resolution_id   TEXT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_churn_hint_resolutions_hint ON zdx_churn_hint_resolutions (hint_id);
