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
INSERT INTO zdx_tests (project_id, component, name, layer, status, duration_ms, last_run_at,
                       last_run_branch, last_run_sha, last_failed_at, last_failed_branch)
VALUES (@project_id, @component, @name, @layer, @status, @duration_ms, NOW(),
        @last_run_branch, @last_run_sha,
        CASE WHEN @status::text = 'fail' THEN NOW() ELSE NULL END,
        CASE WHEN @status::text = 'fail' THEN @last_run_branch::text ELSE '' END)
ON CONFLICT (project_id, component, name) DO UPDATE
SET layer              = EXCLUDED.layer,
    status             = EXCLUDED.status,
    duration_ms        = EXCLUDED.duration_ms,
    last_run_at        = NOW(),
    last_run_branch    = EXCLUDED.last_run_branch,
    last_run_sha       = EXCLUDED.last_run_sha,
    last_failed_at     = CASE WHEN EXCLUDED.status = 'fail' THEN NOW() ELSE zdx_tests.last_failed_at END,
    last_failed_branch = CASE WHEN EXCLUDED.status = 'fail' THEN EXCLUDED.last_run_branch ELSE zdx_tests.last_failed_branch END
RETURNING id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at,
          last_run_branch, last_run_sha, last_failed_at, last_failed_branch;

-- name: GetTest :one
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at,
       last_run_branch, last_run_sha, last_failed_at, last_failed_branch
FROM zdx_tests WHERE project_id = @project_id AND component = @component AND name = @name;

-- name: GetTestByID :one
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at,
       last_run_branch, last_run_sha, last_failed_at, last_failed_branch
FROM zdx_tests WHERE project_id = @project_id AND id = @id;

-- name: ListTests :many
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at,
       last_run_branch, last_run_sha, last_failed_at, last_failed_branch
FROM zdx_tests WHERE project_id = $1 ORDER BY component, name;


-- name: ListTestsByLayer :many
SELECT id, project_id, component, name, layer, status, duration_ms, last_run_at, created_at,
       last_run_branch, last_run_sha, last_failed_at, last_failed_branch
FROM zdx_tests WHERE project_id = $1 AND layer = $2 ORDER BY component, name;

-- name: ListTestNamesBySpecImportance :many
SELECT DISTINCT t.name
FROM zdx_tests t
JOIN zdx_spec_tests st ON st.test_id = t.id
JOIN zdx_specs s ON s.id = st.spec_id
WHERE t.project_id = $1 AND s.importance = $2
ORDER BY t.name;

-- name: ListTestsForSpec :many
SELECT t.id, t.project_id, t.component, t.name, t.layer, t.status, t.duration_ms, t.last_run_at, t.created_at,
       t.last_run_branch, t.last_run_sha, t.last_failed_at, t.last_failed_branch
FROM zdx_tests t
JOIN zdx_spec_tests st ON st.test_id = t.id
WHERE st.spec_id = $1 ORDER BY t.component, t.name;

-- name: ListTestResultHistory :many
-- metaquery: off
SELECT id, driver, test_name, status, duration_ms, run_at, branch, git_sha
FROM zdx_test_result_history
WHERE project_id = @project_id AND test_name = @test_name
  AND (@branch_filter::text = '' OR branch = @branch_filter)
ORDER BY run_at DESC
LIMIT @max_results;

-- name: ListSpecsCoveredByTest :many
-- Used to show what breaks if a test is deleted.
SELECT s.id, s.feature_id, s.description, s.importance
FROM zdx_specs s
JOIN zdx_spec_tests st ON st.spec_id = s.id
WHERE st.test_id = $1 ORDER BY s.id;

-- name: GetFeatureCoverage :many
SELECT
  s.id AS spec_id,
  bool_or(t.layer = 'unit')        AS has_unit,
  bool_or(t.layer = 'integration') AS has_integration,
  bool_or(t.layer = 'demo')        AS has_demo
FROM zdx_specs s
LEFT JOIN zdx_spec_tests st ON st.spec_id = s.id
LEFT JOIN zdx_tests t       ON t.id = st.test_id
WHERE s.feature_id = $1
GROUP BY s.id;

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
INSERT INTO zdx_test_demos (test_id, demo_type, artifact_path, file_id, recorded_branch, recorded_sha)
VALUES (@test_id, @demo_type, @artifact_path, @file_id, @recorded_branch, @recorded_sha)
ON CONFLICT (test_id, demo_type, artifact_path) DO UPDATE
SET file_id = EXCLUDED.file_id,
    recorded_branch = EXCLUDED.recorded_branch,
    recorded_sha = EXCLUDED.recorded_sha
RETURNING id, test_id, demo_type, artifact_path, file_id, recorded_branch, recorded_sha, created_at;

-- name: ListTestDemos :many
SELECT id, test_id, demo_type, artifact_path, file_id, recorded_branch, recorded_sha, created_at
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
       td.recorded_branch, td.recorded_sha,
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
-- metaquery: off
-- Specs linked to tests but where none of those tests are demo-component tests
-- (TestDemo* prefix) and none have recorded demo artifacts. Non-deferred specs only.
SELECT s.id, s.feature_id, s.description, s.importance, f.name AS feature_name
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
JOIN zdx_spec_tests st ON st.spec_id = s.id
JOIN zdx_tests t ON t.id = st.test_id
LEFT JOIN zdx_test_demos td ON td.test_id = st.test_id
WHERE f.project_id = $1
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = s.id AND i.status = 'open'
  )
GROUP BY s.id, s.feature_id, s.description, s.importance, f.name
HAVING COUNT(td.id) = 0 AND COUNT(CASE WHEN t.component = 'demo' THEN 1 END) = 0
ORDER BY f.name, s.id;
