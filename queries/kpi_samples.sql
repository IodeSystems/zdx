-- name: InsertKpiSample :one
INSERT INTO zdx_kpi_samples (project_id, scope, check_name, value, unit)
VALUES (@project_id, @scope, @check_name, @value, @unit)
RETURNING id, project_id, sampled_at, scope, check_name, value, unit;

-- name: ListKpiTrend :many
SELECT id, project_id, sampled_at, scope, check_name, value, unit
FROM zdx_kpi_samples
WHERE project_id = @project_id
  AND scope = @scope
  AND check_name = @check_name
ORDER BY sampled_at DESC
LIMIT @n;

-- name: LatestTwoKPISamplesPerCheck :many
WITH ranked AS (
  SELECT id, project_id, sampled_at, scope, check_name, value, unit,
    row_number() OVER (PARTITION BY check_name ORDER BY sampled_at DESC) AS rn
  FROM zdx_kpi_samples
  WHERE project_id = @project_id AND scope = @scope
)
SELECT id, project_id, sampled_at, scope, check_name, value, unit
FROM ranked WHERE rn <= 2
ORDER BY check_name, rn;
