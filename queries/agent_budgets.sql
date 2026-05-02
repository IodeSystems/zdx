-- name: UpsertAgentBudget :one
INSERT INTO zdx_agent_budgets (agent_id, token_ceiling, cost_ceiling)
VALUES (@agent_id, @token_ceiling, @cost_ceiling)
ON CONFLICT (agent_id) WHERE agent_id IS NOT NULL
DO UPDATE SET token_ceiling = EXCLUDED.token_ceiling,
             cost_ceiling   = EXCLUDED.cost_ceiling,
             updated_at     = NOW()
RETURNING *;

-- name: UpsertProjectBudget :one
INSERT INTO zdx_agent_budgets (project_id, token_ceiling, cost_ceiling)
VALUES (@project_id, @token_ceiling, @cost_ceiling)
ON CONFLICT (project_id) WHERE project_id IS NOT NULL AND agent_id IS NULL
DO UPDATE SET token_ceiling = EXCLUDED.token_ceiling,
             cost_ceiling   = EXCLUDED.cost_ceiling,
             updated_at     = NOW()
RETURNING *;

-- name: GetAgentBudget :one
SELECT * FROM zdx_agent_budgets WHERE agent_id = @agent_id;

-- name: GetProjectBudget :one
SELECT * FROM zdx_agent_budgets WHERE project_id = @project_id AND agent_id IS NULL;

-- name: ListBudgets :many
SELECT * FROM zdx_agent_budgets ORDER BY id;

-- name: GetAgentTokenUsage :one
SELECT
  coalesce(sum((event_json->'message'->'usage'->>'input_tokens')::bigint), 0)::bigint           AS input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'output_tokens')::bigint), 0)::bigint          AS output_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_read_input_tokens')::bigint), 0)::bigint AS cache_read_input_tokens,
  coalesce(sum((event_json->'message'->'usage'->>'cache_creation_input_tokens')::bigint), 0)::bigint AS cache_creation_input_tokens
FROM zdx_claude_events
WHERE agent_id = @agent_id
  AND event_type = 'assistant'
  AND event_json->'message'->'usage' IS NOT NULL;

-- name: RecordBudgetPause :one
INSERT INTO zdx_budget_pauses (agent_id, project_id, reason)
VALUES (@agent_id, @project_id, @reason)
RETURNING *;

-- name: GetActiveBudgetPause :one
SELECT * FROM zdx_budget_pauses
WHERE agent_id = @agent_id AND lifted_at IS NULL
ORDER BY paused_at DESC
LIMIT 1;

-- name: LiftBudgetPause :exec
UPDATE zdx_budget_pauses
SET lifted_at = NOW(), lifted_by = @lifted_by
WHERE agent_id = @agent_id AND lifted_at IS NULL;
