-- name: GetLLMConfig :one
SELECT id, type, url, model, api_key, agent_type, model_low, model_medium, model_high, created_at
FROM zdx_llm_configs LIMIT 1;

-- name: UpsertLLMConfig :one
INSERT INTO zdx_llm_configs (id, type, url, model, api_key, agent_type, model_low, model_medium, model_high)
VALUES (TRUE, @type, @url, @model, @api_key, @agent_type, @model_low, @model_medium, @model_high)
ON CONFLICT (id) DO UPDATE
SET type         = EXCLUDED.type,
    url          = EXCLUDED.url,
    model        = EXCLUDED.model,
    api_key      = EXCLUDED.api_key,
    agent_type   = EXCLUDED.agent_type,
    model_low    = EXCLUDED.model_low,
    model_medium = EXCLUDED.model_medium,
    model_high   = EXCLUDED.model_high
RETURNING id, type, url, model, api_key, agent_type, model_low, model_medium, model_high, created_at;
