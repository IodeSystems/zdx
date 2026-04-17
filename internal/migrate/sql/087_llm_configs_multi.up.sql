-- Allow multiple LLM configs with priority-based fallthrough.
-- Replace singleton (id BOOLEAN PRIMARY KEY = TRUE) with a serial id,
-- add name + priority, rename model -> embedding_model, make level slots
-- nullable (NULL = no override, walk to next config), and prohibit
-- embedding_model when agent_type='claude'.

ALTER TABLE zdx_llm_configs DROP CONSTRAINT zdx_llm_configs_singleton;
ALTER TABLE zdx_llm_configs DROP COLUMN id;
ALTER TABLE zdx_llm_configs ADD COLUMN id BIGSERIAL PRIMARY KEY;

ALTER TABLE zdx_llm_configs ADD COLUMN name TEXT NOT NULL DEFAULT 'default';
ALTER TABLE zdx_llm_configs ADD COLUMN priority INT;
UPDATE zdx_llm_configs SET priority = 1;
ALTER TABLE zdx_llm_configs ALTER COLUMN priority SET NOT NULL;
ALTER TABLE zdx_llm_configs
    ADD CONSTRAINT zdx_llm_configs_priority_uniq UNIQUE (priority)
    DEFERRABLE INITIALLY IMMEDIATE;

ALTER TABLE zdx_llm_configs RENAME COLUMN model TO embedding_model;
ALTER TABLE zdx_llm_configs
    ALTER COLUMN embedding_model DROP NOT NULL,
    ALTER COLUMN embedding_model DROP DEFAULT;
UPDATE zdx_llm_configs SET embedding_model = NULL WHERE embedding_model = '';

ALTER TABLE zdx_llm_configs
    ALTER COLUMN model_low DROP NOT NULL,
    ALTER COLUMN model_low DROP DEFAULT,
    ALTER COLUMN model_medium DROP NOT NULL,
    ALTER COLUMN model_medium DROP DEFAULT,
    ALTER COLUMN model_high DROP NOT NULL,
    ALTER COLUMN model_high DROP DEFAULT;
UPDATE zdx_llm_configs SET model_low    = NULL WHERE model_low    = '';
UPDATE zdx_llm_configs SET model_medium = NULL WHERE model_medium = '';
UPDATE zdx_llm_configs SET model_high   = NULL WHERE model_high   = '';

ALTER TABLE zdx_llm_configs
    ADD CONSTRAINT zdx_llm_configs_claude_no_embedding
        CHECK (agent_type <> 'claude' OR embedding_model IS NULL);
