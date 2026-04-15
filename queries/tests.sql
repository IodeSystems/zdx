-- name: UpsertTestResult :exec
INSERT INTO zdx_test_results (project_id, driver, test_name, feature, status, duration_ms)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms)
ON CONFLICT (project_id, driver, test_name) DO UPDATE
SET status = EXCLUDED.status, duration_ms = EXCLUDED.duration_ms, run_at = NOW(),
    feature = EXCLUDED.feature;

-- name: InsertTestResultHistory :exec
INSERT INTO zdx_test_result_history (project_id, driver, test_name, feature, status, duration_ms)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms);

-- name: UpsertTest :one
INSERT INTO zdx_tests (project_id, component, name, layer, status, duration_ms, last_run_at)
VALUES (@project_id, @component, @name, @layer, @status, @duration_ms, NOW())
ON CONFLICT (project_id, component, name) DO UPDATE
SET layer       = EXCLUDED.layer,
    status      = EXCLUDED.status,
    duration_ms = EXCLUDED.duration_ms,
    last_run_at = NOW()
RETURNING id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at;

-- name: GetTest :one
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at
FROM zdx_tests WHERE project_id = @project_id AND component = @component AND name = @name;

-- name: GetTestByID :one
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at
FROM zdx_tests WHERE project_id = @project_id AND id = @id;

-- name: ListTests :many
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at
FROM zdx_tests WHERE project_id = $1 ORDER BY component, name;

-- name: CountTests :one
SELECT count(*) FROM zdx_tests WHERE project_id = $1;

-- name: ListTestsPaginated :many
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at
FROM zdx_tests WHERE project_id = $1 ORDER BY component, name
LIMIT $2 OFFSET $3;

-- name: ListTestsByLayer :many
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at
FROM zdx_tests WHERE project_id = $1 AND layer = $2 ORDER BY component, name;

-- name: ListTestsForSpec :many
SELECT t.id, t.project_id, t.component, t.name, t.layer, t.status, t.duration_ms, t.last_run_at, t.created_at
FROM zdx_tests t
JOIN zdx_spec_tests st ON st.test_id = t.id
WHERE st.spec_id = $1 ORDER BY t.component, t.name;

-- name: ListTestResultHistory :many
SELECT id, driver, test_name, status, duration_ms, run_at
FROM zdx_test_result_history
WHERE project_id = @project_id AND test_name = @test_name
ORDER BY run_at DESC
LIMIT @max_results;

-- name: ListSpecsCoveredByTest :many
-- Used to show what breaks if a test is deleted.
SELECT s.id, s.feature_id, s.description, s.kind
FROM zdx_specs s
JOIN zdx_spec_tests st ON st.spec_id = s.id
WHERE st.test_id = $1 ORDER BY s.id;

-- name: LinkSpecTest :exec
INSERT INTO zdx_spec_tests (spec_id, test_id)
VALUES (@spec_id, @test_id)
ON CONFLICT DO NOTHING;

-- name: UnlinkSpecTest :exec
DELETE FROM zdx_spec_tests WHERE spec_id = @spec_id AND test_id = @test_id;

-- name: DeleteTest :exec
-- Will fail at the DB level (RESTRICT) if spec_tests rows reference this test.
-- Call ListSpecsCoveredByTest first to surface what breaks.
DELETE FROM zdx_tests WHERE id = $1;
