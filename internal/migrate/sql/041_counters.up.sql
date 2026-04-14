CREATE TABLE zdx_counted (
    id           bigserial PRIMARY KEY,
    project_id   integer REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component    text DEFAULT '' NOT NULL,
    environment  text DEFAULT '' NOT NULL,
    name         text NOT NULL,
    value        integer NOT NULL,
    source       text DEFAULT '' NOT NULL,
    context_json jsonb DEFAULT '{}' NOT NULL,
    count        integer DEFAULT 1 NOT NULL,
    total_value  bigint DEFAULT 0 NOT NULL,
    created_at   timestamptz DEFAULT now() NOT NULL
);

CREATE UNIQUE INDEX zdx_counted_name ON zdx_counted (COALESCE(project_id, 0), component, environment, name);
CREATE INDEX zdx_counted_project_created ON zdx_counted (project_id, created_at);
CREATE INDEX zdx_counted_context_gin ON zdx_counted USING GIN (context_json jsonb_path_ops);

CREATE TABLE zdx_counter_events (
    id           bigserial PRIMARY KEY,
    project_id   integer REFERENCES zdx_projects(id) ON DELETE CASCADE,
    component    text DEFAULT '' NOT NULL,
    environment  text DEFAULT '' NOT NULL,
    name         text NOT NULL,
    value        integer NOT NULL,
    source       text DEFAULT '' NOT NULL,
    context_json jsonb DEFAULT '{}' NOT NULL,
    created_at   timestamptz DEFAULT now() NOT NULL
);

CREATE INDEX zdx_counter_events_project_created ON zdx_counter_events (project_id, created_at);
CREATE INDEX zdx_counter_events_context_gin ON zdx_counter_events USING GIN (context_json jsonb_path_ops);
