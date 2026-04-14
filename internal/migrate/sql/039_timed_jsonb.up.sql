ALTER TABLE zdx_timed ALTER COLUMN context_json DROP DEFAULT;
ALTER TABLE zdx_timed ALTER COLUMN context_json TYPE JSONB USING context_json::jsonb;
ALTER TABLE zdx_timed ALTER COLUMN context_json SET DEFAULT '{}'::jsonb;

CREATE INDEX zdx_timed_context_gin ON zdx_timed USING GIN (context_json jsonb_path_ops);
