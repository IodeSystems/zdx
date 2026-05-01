-- name: InsertJournalEntry :one
INSERT INTO zdx_journal_entries (project_id, role, date, tldr, assessment, concerns, next, state_json, changelog_json, kpi_delta_json, needs_review)
VALUES (@project_id, @role, @date, @tldr, @assessment, @concerns, @next, @state_json, @changelog_json, @kpi_delta_json, @needs_review)
RETURNING id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, kpi_delta_json, needs_review, created_at;

-- name: ListJournalEntries :many
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, kpi_delta_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC;

-- name: GetLatestJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, kpi_delta_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 1;

-- name: GetUnreviewedJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, kpi_delta_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 AND needs_review = true ORDER BY date DESC LIMIT 1;

-- name: GetJournalEntryByID :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, kpi_delta_json, needs_review, created_at
FROM zdx_journal_entries WHERE id = $1 AND project_id = $2;

-- name: MarkJournalEntryReviewed :exec
UPDATE zdx_journal_entries SET needs_review = false WHERE id = $1;

-- name: StandupOwnerYield :one
SELECT
  (SELECT count(*) FROM zdx_issues    i WHERE i.project_id = $1 AND i.updated_at > NOW() - INTERVAL '30 days' AND (i.closed_at IS NULL OR i.closed_at < NOW() - INTERVAL '30 days')) AS issues_touched_not_closed,
  (SELECT count(*) FROM zdx_issues    i WHERE i.project_id = $1 AND i.closed_at > NOW() - INTERVAL '30 days')                                                                         AS closed_30d,
  (SELECT count(*) FROM zdx_features  f WHERE f.project_id = $1 AND f.goal_id IS NOT NULL AND f.metric_name = '')                                                                     AS features_without_metric;

-- name: StandupTopReopenedIssues :many
SELECT id, title, reopen_count FROM zdx_issues
WHERE project_id = $1 AND reopen_count > 0
ORDER BY reopen_count DESC LIMIT 3;

-- name: StandupTechYield :one
SELECT
  (SELECT count(*) FROM zdx_claude_sessions s WHERE s.project_id = $1 AND s.created_at >= $2) AS sessions_in_period,
  (SELECT count(*) FROM zdx_issues          i WHERE i.project_id = $1 AND i.closed_at  >= $2) AS closed_in_period;

-- name: StandupTopThrashingIssues :many
-- Top issues by claude session count in the period — surfaces which issues are consuming the most
-- agent attention, useful when sessions/closed is high (agent thrashing breach).
SELECT issue_id, count(*) AS session_count
FROM zdx_claude_sessions
WHERE project_id = $1 AND created_at >= $2 AND issue_id != ''
GROUP BY issue_id
ORDER BY session_count DESC
LIMIT 3;

-- name: StandupSpecDelta :one
-- Spec activity in a 30d window for the project's specs (joined via zdx_features → project_id).
-- specs_added: distinct specs whose first issue link landed in the window. zdx_specs has no
--   created_at column, so the earliest spec-issue link is used as a proxy for "newly tracked".
-- specs_covered: specs linked to issues that closed in the window.
-- specs_deferred: spec deferrals created in the window.
-- covered_spec_ids / deferred_spec_ids: arrays of spec IDs for the body of yield alerts.
SELECT
  (SELECT count(*) FROM (
     SELECT si.spec_id, MIN(si.created_at) AS first_link
     FROM zdx_spec_issues si
     JOIN zdx_specs    s ON s.id = si.spec_id
     JOIN zdx_features f ON f.id = s.feature_id
     WHERE f.project_id = $1
     GROUP BY si.spec_id
   ) firsts WHERE firsts.first_link > NOW() - INTERVAL '30 days')              AS specs_added,
  (SELECT count(DISTINCT s.id)
     FROM zdx_specs       s
     JOIN zdx_features    f  ON f.id  = s.feature_id
     JOIN zdx_spec_issues si ON si.spec_id = s.id
     JOIN zdx_issues      i  ON i.id  = si.issue_id
     WHERE f.project_id = $1
       AND i.closed_at > NOW() - INTERVAL '30 days')                            AS specs_covered,
  (SELECT count(*)
     FROM zdx_spec_deferrals sd
     JOIN zdx_specs    s ON s.id = sd.spec_id
     JOIN zdx_features f ON f.id = s.feature_id
     WHERE f.project_id = $1
       AND sd.created_at > NOW() - INTERVAL '30 days')                          AS specs_deferred,
  (SELECT array_agg(DISTINCT s.id ORDER BY s.id)
     FROM zdx_specs       s
     JOIN zdx_features    f  ON f.id  = s.feature_id
     JOIN zdx_spec_issues si ON si.spec_id = s.id
     JOIN zdx_issues      i  ON i.id  = si.issue_id
     WHERE f.project_id = $1
       AND i.closed_at > NOW() - INTERVAL '30 days')                            AS covered_spec_ids,
  (SELECT array_agg(sd.spec_id ORDER BY sd.spec_id)
     FROM zdx_spec_deferrals sd
     JOIN zdx_specs    s ON s.id = sd.spec_id
     JOIN zdx_features f ON f.id = s.feature_id
     WHERE f.project_id = $1
       AND sd.created_at > NOW() - INTERVAL '30 days')                          AS deferred_spec_ids;

-- name: JournalVelocity :one
-- closed_at is set by CloseIssue and cleared by ReopenIssue/ReadyIssue, so
-- filtering by closed_at measures actual close events — updated_at conflates
-- every edit (retriage, comment, resolution add) with the close event.
SELECT
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.closed_at > NOW() - INTERVAL '7 days')  AS closed_7d,
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.closed_at > NOW() - INTERVAL '14 days') AS closed_14d,
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.closed_at > NOW() - INTERVAL '30 days') AS closed_30d,
  (SELECT count(*) FROM zdx_features f WHERE f.project_id = $1 AND f.goal_id IS NULL)                        AS features_without_goal,
  (SELECT count(*) FROM zdx_focuses  fo WHERE fo.project_id = $1 AND fo.status = 'active')                   AS active_focus_count;

-- ── Owner Snapshot (IS-589) ──────────────────────────────────────────────────
-- Tracker-state metrics surfaced to the dx standup checkin --role=owner flow.
-- Split into 4 queries for readability; the handler aggregates them into one
-- map[string]float64 payload.

-- name: OwnerSnapshotFeatures :one
-- features_total / features_with_specs / features_with_demos counts.
-- A "demo" is any zdx_test_demos row attached (via zdx_spec_tests) to a spec
-- of the feature.
SELECT
  (SELECT count(*)
     FROM zdx_features f
     WHERE f.project_id = $1)::bigint                                   AS features_total,
  (SELECT count(DISTINCT s.feature_id)
     FROM zdx_specs s
     JOIN zdx_features f ON f.id = s.feature_id
     WHERE f.project_id = $1)::bigint                                   AS features_with_specs,
  (SELECT count(DISTINCT f.id)
     FROM zdx_features f
     JOIN zdx_specs s ON s.feature_id = f.id
     JOIN zdx_spec_tests st ON st.spec_id = s.id
     JOIN zdx_test_demos td ON td.test_id = st.test_id
     WHERE f.project_id = $1)::bigint                                   AS features_with_demos;

-- name: OwnerSnapshotSpecCoverage :one
-- Spec coverage by importance tier. A spec is "implemented" when at least one
-- linked issue has closed_at IS NOT NULL.
SELECT
  count(*) FILTER (WHERE s.importance = 'must')::bigint                                                     AS specs_must_total,
  count(*) FILTER (WHERE s.importance = 'must' AND EXISTS (
    SELECT 1 FROM zdx_spec_issues si JOIN zdx_issues i ON i.id = si.issue_id
    WHERE si.spec_id = s.id AND i.closed_at IS NOT NULL
  ))::bigint                                                                                                AS specs_must_implemented,
  count(*) FILTER (WHERE s.importance = 'should')::bigint                                                   AS specs_should_total,
  count(*) FILTER (WHERE s.importance = 'should' AND EXISTS (
    SELECT 1 FROM zdx_spec_issues si JOIN zdx_issues i ON i.id = si.issue_id
    WHERE si.spec_id = s.id AND i.closed_at IS NOT NULL
  ))::bigint                                                                                                AS specs_should_implemented,
  count(*) FILTER (WHERE s.importance = 'nice')::bigint                                                     AS specs_nice_total,
  count(*) FILTER (WHERE s.importance = 'nice' AND EXISTS (
    SELECT 1 FROM zdx_spec_issues si JOIN zdx_issues i ON i.id = si.issue_id
    WHERE si.spec_id = s.id AND i.closed_at IS NOT NULL
  ))::bigint                                                                                                AS specs_nice_implemented
FROM zdx_specs s JOIN zdx_features f ON f.id = s.feature_id
WHERE f.project_id = $1;

-- name: OwnerSnapshotPeriod :one
-- Velocity counters in a configurable window (default caller passes 30).
SELECT
  (SELECT count(DISTINCT s.id)
     FROM zdx_specs s
     JOIN zdx_features    f  ON f.id  = s.feature_id
     JOIN zdx_spec_issues si ON si.spec_id = s.id
     JOIN zdx_issues      i  ON i.id  = si.issue_id
     WHERE f.project_id = $1
       AND i.closed_at > NOW() - (@period_days::int || ' days')::interval)::bigint AS specs_implemented_in_period,
  (SELECT count(*)
     FROM zdx_issues i
     WHERE i.project_id = $1
       AND i.closed_at > NOW() - (@period_days::int || ' days')::interval)::bigint AS issues_closed_in_period;

-- name: OwnerSnapshotMaturity :one
-- Maturity rungs: count of zdx_maturity_items by terminal status.
-- 'done' counts as passed; 'open' counts as failed (still owed).
SELECT
  count(*) FILTER (WHERE status = 'done')::bigint AS rungs_passed,
  count(*) FILTER (WHERE status = 'open')::bigint AS rungs_failed
FROM zdx_maturity_items
WHERE project_id = $1;

-- name: OwnerSnapshotBlockerResolution :one
-- Average minutes from blocker_question creation to answer for resolved
-- (status='answered') questions. Returns 0 when no resolved questions exist.
SELECT
  COALESCE(
    AVG(EXTRACT(EPOCH FROM (answered_at - created_at)) / 60.0),
    0
  )::float8 AS avg_resolution_minutes
FROM zdx_blocker_questions
WHERE project_id = $1
  AND status = 'answered'
  AND answered_at IS NOT NULL;
