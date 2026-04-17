-- Revert to singleton. Keeps only the lowest-priority row if multiple exist.
DELETE FROM zdx_llm_configs
WHERE id NOT IN (
    SELECT id FROM zdx_llm_configs ORDER BY priority ASC LIMIT 1
);

ALTER TABLE zdx_llm_configs DROP CONSTRAINT zdx_llm_configs_claude_no_embedding;
ALTER TABLE zdx_llm_configs DROP CONSTRAINT zdx_llm_configs_priority_uniq;

UPDATE zdx_llm_configs SET embedding_model = '' WHERE embedding_model IS NULL;
UPDATE zdx_llm_configs SET model_low    = '' WHERE model_low    IS NULL;
UPDATE zdx_llm_configs SET model_medium = '' WHERE model_medium IS NULL;
UPDATE zdx_llm_configs SET model_high   = '' WHERE model_high   IS NULL;

ALTER TABLE zdx_llm_configs
    ALTER COLUMN embedding_model SET DEFAULT '',
    ALTER COLUMN embedding_model SET NOT NULL,
    ALTER COLUMN model_low SET DEFAULT '',
    ALTER COLUMN model_low SET NOT NULL,
    ALTER COLUMN model_medium SET DEFAULT '',
    ALTER COLUMN model_medium SET NOT NULL,
    ALTER COLUMN model_high SET DEFAULT '',
    ALTER COLUMN model_high SET NOT NULL;

ALTER TABLE zdx_llm_configs RENAME COLUMN embedding_model TO model;

ALTER TABLE zdx_llm_configs DROP COLUMN priority;
ALTER TABLE zdx_llm_configs DROP COLUMN name;
ALTER TABLE zdx_llm_configs DROP COLUMN id;
ALTER TABLE zdx_llm_configs ADD COLUMN id BOOLEAN PRIMARY KEY DEFAULT TRUE;
ALTER TABLE zdx_llm_configs
    ADD CONSTRAINT zdx_llm_configs_singleton CHECK (id = TRUE);
