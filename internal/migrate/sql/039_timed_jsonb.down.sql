DROP INDEX IF EXISTS zdx_timed_context_gin;

ALTER TABLE zdx_timed
  ALTER COLUMN context_json TYPE TEXT USING context_json::text,
  ALTER COLUMN context_json SET DEFAULT '{}'::text;
