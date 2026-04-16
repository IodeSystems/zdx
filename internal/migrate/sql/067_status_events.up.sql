CREATE TABLE zdx_status_events (
    id           bigserial PRIMARY KEY,
    project_id   integer     NOT NULL,
    target_type  text        NOT NULL CHECK (target_type IN ('issue', 'task')),
    target_id    text        NOT NULL,
    from_status  text        NOT NULL DEFAULT '',
    to_status    text        NOT NULL,
    agent_id     text        NOT NULL DEFAULT '',
    session_id   text        NOT NULL DEFAULT '',
    user_id      text        NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX zdx_status_events_target_idx
    ON zdx_status_events (target_type, target_id, created_at DESC);

CREATE INDEX zdx_status_events_project_idx
    ON zdx_status_events (project_id, created_at DESC);
