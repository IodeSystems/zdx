CREATE TABLE zdx_session_audit_events (
  id            BIGSERIAL PRIMARY KEY,
  session_pk    BIGINT NOT NULL REFERENCES zdx_claude_sessions(id) ON DELETE CASCADE,
  agent_id      TEXT NOT NULL DEFAULT '',
  todo_id       INT REFERENCES zdx_todos(id) ON DELETE SET NULL,
  turn_id       TEXT NOT NULL DEFAULT '',
  event_type    TEXT NOT NULL,
  payload       JSONB NOT NULL DEFAULT '{}',
  occurred_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX zdx_session_audit_events_session_pk_idx ON zdx_session_audit_events(session_pk);
CREATE INDEX zdx_session_audit_events_agent_id_idx ON zdx_session_audit_events(agent_id);
