-- name: UpsertTestResult :exec
INSERT INTO zdx_test_results (project_id, driver, test_name, feature, status, duration_ms, branch, git_sha)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms, @branch, @git_sha)
ON CONFLICT (project_id, driver, test_name) DO UPDATE
SET status = EXCLUDED.status, duration_ms = EXCLUDED.duration_ms, run_at = NOW(),
    feature = EXCLUDED.feature, branch = EXCLUDED.branch, git_sha = EXCLUDED.git_sha;

-- name: InsertTestResultHistory :exec
INSERT INTO zdx_test_result_history (project_id, driver, test_name, feature, status, duration_ms, branch, git_sha)
VALUES (@project_id, @driver, @test_name, @feature, @status, @duration_ms, @branch, @git_sha);

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
SELECT id, driver, test_name, status, duration_ms, run_at, branch, git_sha
FROM zdx_test_result_history
WHERE project_id = @project_id AND test_name = @test_name
  AND (@branch_filter::text = '' OR branch = @branch_filter)
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

-- name: UpsertTestDemo :one
INSERT INTO zdx_test_demos (test_id, demo_type, artifact_path, file_id)
VALUES (@test_id, @demo_type, @artifact_path, @file_id)
ON CONFLICT (test_id, demo_type, artifact_path) DO UPDATE
SET file_id = EXCLUDED.file_id
RETURNING id, test_id, demo_type, artifact_path, file_id, created_at;

-- name: ListTestDemos :many
SELECT id, test_id, demo_type, artifact_path, file_id, created_at
FROM zdx_test_demos WHERE test_id = $1 ORDER BY demo_type, artifact_path;

-- name: ListDemosForSpec :many
-- All demo artifacts attached to tests linked to the given spec. file_id falls
-- back to sibling rows sharing the same (demo_type, artifact_path) — see
-- GetDemoByID for rationale.
SELECT td.id, td.test_id, td.demo_type, td.artifact_path,
       COALESCE(td.file_id, (
           SELECT sib.file_id FROM zdx_test_demos sib
           WHERE sib.demo_type = td.demo_type
             AND sib.artifact_path = td.artifact_path
             AND sib.file_id IS NOT NULL
           LIMIT 1
       )) AS file_id,
       t.component AS test_component, t.name AS test_name
FROM zdx_test_demos td
JOIN zdx_spec_tests st ON st.test_id = td.test_id
JOIN zdx_tests t ON t.id = td.test_id
WHERE st.spec_id = $1
ORDER BY td.demo_type, t.name;

-- name: GetDemoByID :one
-- file_id falls back to any sibling row with the same (demo_type, artifact_path)
-- that does have a file_id, so legacy rows linked to non-recorder tests still
-- resolve to the uploaded artifact instead of 404ing on handleServeDemo.
SELECT td.id, td.test_id, td.demo_type, td.artifact_path,
       COALESCE(td.file_id, (
           SELECT sib.file_id FROM zdx_test_demos sib
           WHERE sib.demo_type = td.demo_type
             AND sib.artifact_path = td.artifact_path
             AND sib.file_id IS NOT NULL
           LIMIT 1
       )) AS file_id,
       t.component AS test_component, t.name AS test_name,
       t.status AS test_status, t.duration_ms AS test_duration_ms,
       t.project_id AS project_id
FROM zdx_test_demos td
JOIN zdx_tests t ON t.id = td.test_id
WHERE td.id = $1;

-- name: ListDemos :many
-- All demo artifacts in the project, joined to their owning test. file_id falls
-- back to sibling rows sharing the same (demo_type, artifact_path) — see
-- GetDemoByID for rationale.
SELECT td.id, td.test_id, td.demo_type, td.artifact_path,
       COALESCE(td.file_id, (
           SELECT sib.file_id FROM zdx_test_demos sib
           WHERE sib.demo_type = td.demo_type
             AND sib.artifact_path = td.artifact_path
             AND sib.file_id IS NOT NULL
           LIMIT 1
       )) AS file_id,
       t.component AS test_component, t.name AS test_name,
       t.status AS test_status, t.duration_ms AS test_duration_ms
FROM zdx_test_demos td
JOIN zdx_tests t ON t.id = td.test_id
WHERE t.project_id = $1
ORDER BY td.demo_type, t.component, t.name;

-- name: ListSpecsWithoutDemos :many
-- Specs linked to tests but where none of those tests have demo artifacts.
-- Non-deferred specs only.
SELECT s.id, s.feature_id, s.description, s.kind, f.name AS feature_name
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
JOIN zdx_spec_tests st ON st.spec_id = s.id
LEFT JOIN zdx_test_demos td ON td.test_id = st.test_id
WHERE f.project_id = $1
  AND s.deferred = false
GROUP BY s.id, s.feature_id, s.description, s.kind, f.name
HAVING COUNT(td.id) = 0
ORDER BY f.name, s.id;
