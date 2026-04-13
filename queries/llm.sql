-- name: GetLLMConfig :one
SELECT id, type, url, model, api_key, created_at FROM zdx_llm_configs LIMIT 1;

-- name: UpsertLLMConfig :one
INSERT INTO zdx_llm_configs (id, type, url, model, api_key)
VALUES (TRUE, @type, @url, @model, @api_key)
ON CONFLICT (id) DO UPDATE
SET type    = EXCLUDED.type,
    url     = EXCLUDED.url,
    model   = EXCLUDED.model,
    api_key = EXCLUDED.api_key
RETURNING id, type, url, model, api_key, created_at;
