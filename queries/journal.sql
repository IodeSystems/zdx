-- name: InsertJournalEntry :one
INSERT INTO zdx_journal_entries (project_id, role, date, tldr, assessment, concerns, next)
VALUES (@project_id, @role, @date, @tldr, @assessment, @concerns, @next)
RETURNING id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at;

-- name: ListJournalEntries :many
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 20;

-- name: GetLatestJournalEntry :one
SELECT id, project_id, role, date, baseline, tldr, assessment, concerns, next, changelog_json, state_json, created_at
FROM zdx_journal_entries WHERE project_id = $1 AND role = $2 ORDER BY date DESC LIMIT 1;
