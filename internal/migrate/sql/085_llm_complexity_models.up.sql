ALTER TABLE zdx_llm_configs
    ADD COLUMN agent_type   TEXT NOT NULL DEFAULT 'openai',
    ADD COLUMN model_low    TEXT NOT NULL DEFAULT '',
    ADD COLUMN model_medium TEXT NOT NULL DEFAULT '',
    ADD COLUMN model_high   TEXT NOT NULL DEFAULT '';

UPDATE zdx_llm_configs
SET model_medium = model
WHERE model_medium = ''
  AND model <> '';
