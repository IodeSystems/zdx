CREATE TABLE zdx_error_reports (
    id          BIGSERIAL PRIMARY KEY,
    source      TEXT NOT NULL,
    endpoint    TEXT NOT NULL DEFAULT '',
    error_name  TEXT NOT NULL DEFAULT '',
    stack_trace TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_error_reports_created_at ON zdx_error_reports (created_at DESC);
CREATE INDEX idx_error_reports_source     ON zdx_error_reports (source);

CREATE TABLE zdx_slow_queries (
    id           BIGSERIAL PRIMARY KEY,
    sql_hash     TEXT NOT NULL,
    sql_text     TEXT NOT NULL,
    endpoint     TEXT NOT NULL DEFAULT '',
    duration_ms  INT  NOT NULL,
    explain_json TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_slow_queries_endpoint   ON zdx_slow_queries (endpoint);
CREATE INDEX idx_slow_queries_sql_hash   ON zdx_slow_queries (sql_hash);
CREATE INDEX idx_slow_queries_created_at ON zdx_slow_queries (created_at DESC);
