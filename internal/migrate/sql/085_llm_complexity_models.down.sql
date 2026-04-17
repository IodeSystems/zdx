ALTER TABLE zdx_llm_configs
    DROP COLUMN IF EXISTS model_high,
    DROP COLUMN IF EXISTS model_medium,
    DROP COLUMN IF EXISTS model_low,
    DROP COLUMN IF EXISTS agent_type;
