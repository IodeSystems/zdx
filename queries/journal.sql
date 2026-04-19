-- name: InsertJournalEntry :one
INSERT INTO zdx_journal_entries (project_id, role, date, tldr, assessment, concerns, next, state_json, changelog_json, needs_review)
VALUES (@project_id, @role, @date, @tldr, @assessment, @concerns, @next, @state_json, @changelog_json, @needs_review)
RETURNING id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, needs_review, created_at;

-- name: ListJournalEntries :many
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 20;

-- name: GetLatestJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 1;

-- name: GetUnreviewedJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, needs_review, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 AND needs_review = true ORDER BY date DESC LIMIT 1;

-- name: GetJournalEntryByID :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, needs_review, created_at
FROM zdx_journal_entries WHERE id = $1 AND project_id = $2;

-- name: MarkJournalEntryReviewed :exec
UPDATE zdx_journal_entries SET needs_review = false WHERE id = $1;

-- name: JournalVelocity :one
SELECT
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.status = 'closed' AND i.updated_at > NOW() - INTERVAL '7 days')  AS closed_7d,
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.status = 'closed' AND i.updated_at > NOW() - INTERVAL '14 days') AS closed_14d,
  (SELECT count(*) FROM zdx_issues   i WHERE i.project_id = $1 AND i.status = 'closed' AND i.updated_at > NOW() - INTERVAL '30 days') AS closed_30d,
  (SELECT count(*) FROM zdx_features f WHERE f.project_id = $1 AND f.goal_id IS NULL)                                                  AS features_without_goal,
  (SELECT count(*) FROM zdx_focuses  fo WHERE fo.project_id = $1 AND fo.status = 'active')                                            AS active_focus_count;
