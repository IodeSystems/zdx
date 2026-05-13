-- name: ListLLMConfigs :many
SELECT id, name, priority, type, url, embedding_model, api_key,
       agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds, created_at
FROM zdx_llm_configs
ORDER BY priority ASC, id ASC;

-- name: GetLLMConfigByID :one
SELECT id, name, priority, type, url, embedding_model, api_key,
       agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds, created_at
FROM zdx_llm_configs
WHERE id = $1;

-- name: GetPrimaryLLMConfigWithEmbedding :one
-- First (lowest-priority) config whose agent_type is not 'claude' and
-- whose embedding_model is set. Used by the server to pick an embedder.
SELECT id, name, priority, type, url, embedding_model, api_key,
       agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds, created_at
FROM zdx_llm_configs
WHERE agent_type <> 'claude'
  AND embedding_model IS NOT NULL
  AND embedding_model <> ''
ORDER BY priority ASC, id ASC
LIMIT 1;

-- name: NextLLMConfigPriority :one
SELECT COALESCE(MAX(priority), 0) + 1 AS next_priority FROM zdx_llm_configs;

-- name: CreateLLMConfig :one
INSERT INTO zdx_llm_configs (
    name, priority, type, url, embedding_model, api_key,
    agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds
)
VALUES (
    @name, @priority, @type, @url, @embedding_model, @api_key,
    @agent_type, @model_low, @model_medium, @model_high, @model_xhigh, @model_max, @timeout_seconds
)
RETURNING id, name, priority, type, url, embedding_model, api_key,
          agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds, created_at;

-- name: UpdateLLMConfig :one
UPDATE zdx_llm_configs
SET name            = @name,
    type            = @type,
    url             = @url,
    embedding_model = @embedding_model,
    api_key         = @api_key,
    agent_type      = @agent_type,
    model_low       = @model_low,
    model_medium    = @model_medium,
    model_high      = @model_high,
    model_xhigh     = @model_xhigh,
    model_max       = @model_max,
    timeout_seconds = @timeout_seconds
WHERE id = @id
RETURNING id, name, priority, type, url, embedding_model, api_key,
          agent_type, model_low, model_medium, model_high, model_xhigh, model_max, timeout_seconds, created_at;

-- name: UpdateLLMConfigPriority :exec
UPDATE zdx_llm_configs SET priority = @priority WHERE id = @id;

-- name: DeleteLLMConfig :exec
DELETE FROM zdx_llm_configs WHERE id = $1;
