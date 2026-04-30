CREATE TABLE zdx_event_streams (
  id                 BIGSERIAL PRIMARY KEY,
  project_id         INT NOT NULL,
  target_type        TEXT NOT NULL,
  target_id          TEXT NOT NULL,
  last_evaluated_at  TIMESTAMPTZ,
  last_evaluated_by  TEXT,
  UNIQUE (project_id, target_type, target_id)
);

CREATE TABLE zdx_event_threads (
  id              BIGSERIAL PRIMARY KEY,
  project_id      INT NOT NULL,
  target_type     TEXT NOT NULL,
  target_id       TEXT NOT NULL,
  root_event_id   BIGINT NOT NULL,
  title           TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX zdx_event_threads_target_idx
  ON zdx_event_threads (project_id, target_type, target_id);

CREATE TABLE zdx_events (
  id                    BIGSERIAL PRIMARY KEY,
  project_id            INT NOT NULL,
  target_type           TEXT NOT NULL,
  target_id             TEXT NOT NULL,
  thread_id             BIGINT,
  event_type            TEXT NOT NULL,
  author                TEXT NOT NULL,
  author_kind           TEXT NOT NULL,
  summary_json          JSONB NOT NULL DEFAULT '{}',
  detail_json           JSONB NOT NULL DEFAULT '{}',
  agent_process_result  JSONB,
  created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX zdx_events_target_created_idx
  ON zdx_events (project_id, target_type, target_id, created_at);
CREATE INDEX zdx_events_thread_idx
  ON zdx_events (thread_id) WHERE thread_id IS NOT NULL;
