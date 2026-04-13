CREATE TABLE zdx_llm_configs (
    id         BOOLEAN PRIMARY KEY DEFAULT TRUE,  -- singleton: only one row allowed
    type       TEXT NOT NULL DEFAULT 'openai',
    url        TEXT NOT NULL DEFAULT '',
    model      TEXT NOT NULL DEFAULT '',
    api_key    TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT zdx_llm_configs_singleton CHECK (id = TRUE)
);
