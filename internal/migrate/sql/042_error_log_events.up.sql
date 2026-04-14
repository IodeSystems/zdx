CREATE TABLE zdx_error_events (
    id           bigserial PRIMARY KEY,
    project_id   integer REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component    text DEFAULT '' NOT NULL,
    environment  text DEFAULT '' NOT NULL,
    name         text NOT NULL,
    message      text DEFAULT '' NOT NULL,
    stack_trace  text DEFAULT '' NOT NULL,
    source       text DEFAULT '' NOT NULL,
    context_json jsonb DEFAULT '{}' NOT NULL,
    created_at   timestamptz DEFAULT now() NOT NULL
);

CREATE INDEX zdx_error_events_project_created ON zdx_error_events (project_id, created_at);
CREATE INDEX zdx_error_events_context_gin ON zdx_error_events USING GIN (context_json jsonb_path_ops);

CREATE TABLE zdx_log_events (
    id           bigserial PRIMARY KEY,
    project_id   integer REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component    text DEFAULT '' NOT NULL,
    environment  text DEFAULT '' NOT NULL,
    level        text DEFAULT 'info' NOT NULL,
    message      text NOT NULL,
    source       text DEFAULT '' NOT NULL,
    context_json jsonb DEFAULT '{}' NOT NULL,
    created_at   timestamptz DEFAULT now() NOT NULL
);

CREATE INDEX zdx_log_events_project_created ON zdx_log_events (project_id, created_at);
CREATE INDEX zdx_log_events_context_gin ON zdx_log_events USING GIN (context_json jsonb_path_ops);
