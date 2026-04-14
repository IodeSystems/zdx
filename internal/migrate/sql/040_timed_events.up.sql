CREATE TABLE zdx_timed_events (
    id          bigserial PRIMARY KEY,
    project_id  integer REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component   text DEFAULT '' NOT NULL,
    environment text DEFAULT '' NOT NULL,
    name        text NOT NULL,
    duration_ms integer NOT NULL,
    source      text DEFAULT '' NOT NULL,
    context_json jsonb DEFAULT '{}' NOT NULL,
    created_at  timestamptz DEFAULT now() NOT NULL
);

CREATE INDEX zdx_timed_events_project_created ON zdx_timed_events (project_id, created_at);
CREATE INDEX zdx_timed_events_context_gin ON zdx_timed_events USING GIN (context_json jsonb_path_ops);
