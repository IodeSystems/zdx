-- name: AddSpecDeferral :exec
INSERT INTO zdx_spec_deferrals (spec_id, issue_id, note)
VALUES ($1, $2, $3)
ON CONFLICT (spec_id, issue_id) DO UPDATE SET note = EXCLUDED.note;

-- name: RemoveSpecDeferral :exec
DELETE FROM zdx_spec_deferrals WHERE spec_id = $1 AND issue_id = $2;

-- name: ListSpecDeferrals :many
SELECT sd.spec_id, sd.issue_id, sd.note, sd.created_at, i.title AS issue_title, i.status AS issue_status
FROM zdx_spec_deferrals sd
JOIN zdx_issues i ON i.id = sd.issue_id
WHERE sd.spec_id = $1
ORDER BY sd.created_at;

-- name: ListDeferredSpecs :many
SELECT DISTINCT s.id, s.feature_id, s.description, s.importance
FROM zdx_specs s
JOIN zdx_spec_deferrals sd ON sd.spec_id = s.id
JOIN zdx_issues i ON i.id = sd.issue_id
WHERE i.status = 'open'
ORDER BY s.id;

-- name: ListDeferredSpecsWithFeatureForProject :many
SELECT DISTINCT s.id, s.feature_id, f.name AS feature_name, s.description, s.importance
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
JOIN zdx_spec_deferrals sd ON sd.spec_id = s.id
JOIN zdx_issues i ON i.id = sd.issue_id
WHERE f.project_id = @project_id
  AND i.status = 'open'
ORDER BY f.name, s.id;

-- name: ListSpecsWithAllBlockersClosed :many
SELECT s.id, s.feature_id, s.description, s.importance
FROM zdx_specs s
WHERE EXISTS (SELECT 1 FROM zdx_spec_deferrals sd WHERE sd.spec_id = s.id)
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = s.id AND i.status = 'open'
  )
ORDER BY s.id;

-- name: IsSpecDeferred :one
SELECT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = $1 AND i.status = 'open'
) AS deferred;

-- name: ListMustSpecDemoGateOffenders :many
-- Must-specs linked to an issue (via tasks→features by name) that are NOT
-- deferred and have no passing demo. A passing demo is a zdx_tests row
-- with status=pass AND (component=demo OR has zdx_test_demos artifact).
SELECT s.id AS spec_id, s.description, f.name AS feature_name
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = @project_id
  AND s.importance = 'must'
  AND EXISTS (
    SELECT 1 FROM zdx_tasks t
    WHERE t.project_id = @project_id AND t.issue = @issue_id AND t.feature = f.name
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = s.id AND i.status = 'open'
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_tests st
    JOIN zdx_tests tt ON tt.id = st.test_id
    WHERE st.spec_id = s.id
      AND tt.status = 'pass'
      AND (tt.component = 'demo' OR EXISTS (
        SELECT 1 FROM zdx_test_demos td WHERE td.test_id = tt.id
      ))
  )
ORDER BY f.name, s.id;

-- name: ListCloseGateOffenders :many
-- Specs linked to an issue (via tasks→features by name) that are NOT deferred
-- by any open issue and lack passing-test coverage. Reason is 'no-tests' if
-- the spec has no zdx_spec_tests rows, otherwise 'failing-tests'.
SELECT s.id AS spec_id, s.description, f.name AS feature_name,
  CASE WHEN NOT EXISTS (SELECT 1 FROM zdx_spec_tests st WHERE st.spec_id = s.id)
       THEN 'no-tests' ELSE 'failing-tests' END AS reason
FROM zdx_specs s
JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = @project_id
  AND EXISTS (
    SELECT 1 FROM zdx_tasks t
    WHERE t.project_id = @project_id AND t.issue = @issue_id AND t.feature = f.name
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_deferrals sd
    JOIN zdx_issues i ON i.id = sd.issue_id
    WHERE sd.spec_id = s.id AND i.status = 'open'
  )
  AND NOT EXISTS (
    SELECT 1 FROM zdx_spec_tests st
    JOIN zdx_tests tt ON tt.id = st.test_id
    WHERE st.spec_id = s.id AND tt.status = 'pass'
  )
ORDER BY f.name, s.id;
