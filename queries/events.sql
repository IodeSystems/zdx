-- name: ListEventsByTarget :many
SELECT id, project_id, target_type, target_id, thread_id, event_type,
       author, author_kind, summary_json, detail_json,
       agent_process_result, created_at
FROM zdx_events
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id
ORDER BY created_at, id;

-- name: ListEventsByThread :many
SELECT id, project_id, target_type, target_id, thread_id, event_type,
       author, author_kind, summary_json, detail_json,
       agent_process_result, created_at
FROM zdx_events
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id
  AND thread_id = @thread_id
ORDER BY created_at, id;

-- name: InsertEvent :one
INSERT INTO zdx_events (
  project_id, target_type, target_id, thread_id, event_type,
  author, author_kind, summary_json, detail_json, agent_process_result
)
VALUES (
  @project_id, @target_type, @target_id, @thread_id, @event_type,
  @author, @author_kind, @summary_json, @detail_json, @agent_process_result
)
RETURNING id, project_id, target_type, target_id, thread_id, event_type,
          author, author_kind, summary_json, detail_json,
          agent_process_result, created_at;

-- name: GetStreamByTarget :one
SELECT id, project_id, target_type, target_id,
       last_evaluated_at, last_evaluated_by
FROM zdx_event_streams
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id;

-- name: UpsertStream :one
INSERT INTO zdx_event_streams (
  project_id, target_type, target_id, last_evaluated_at, last_evaluated_by
)
VALUES (@project_id, @target_type, @target_id, @last_evaluated_at, @last_evaluated_by)
ON CONFLICT (project_id, target_type, target_id)
DO UPDATE SET last_evaluated_at = EXCLUDED.last_evaluated_at,
              last_evaluated_by = EXCLUDED.last_evaluated_by
RETURNING id, project_id, target_type, target_id,
          last_evaluated_at, last_evaluated_by;

-- name: CreateThread :one
INSERT INTO zdx_event_threads (
  project_id, target_type, target_id, root_event_id, title
)
VALUES (@project_id, @target_type, @target_id, @root_event_id, @title)
RETURNING id, project_id, target_type, target_id, root_event_id, title, created_at;

-- name: ListThreadsByTarget :many
SELECT id, project_id, target_type, target_id, root_event_id, title, created_at
FROM zdx_event_threads
WHERE project_id = @project_id
  AND target_type = @target_type
  AND target_id = @target_id
ORDER BY created_at, id;

-- name: GetEventByID :one
SELECT id, project_id, target_type, target_id, thread_id, event_type,
       author, author_kind, summary_json, detail_json,
       agent_process_result, created_at
FROM zdx_events
WHERE id = @id;

-- name: GetThreadByRoot :one
SELECT id, project_id, target_type, target_id, root_event_id, title, created_at
FROM zdx_event_threads
WHERE root_event_id = @root_event_id;

-- name: SetEventVerdict :one
UPDATE zdx_events
SET agent_process_result = @agent_process_result
WHERE id = @id
RETURNING id, project_id, target_type, target_id, thread_id, event_type,
          author, author_kind, summary_json, detail_json,
          agent_process_result, created_at;

-- name: SetThreadTitle :one
UPDATE zdx_event_threads
SET title = @title
WHERE id = @id
RETURNING id, project_id, target_type, target_id, root_event_id, title, created_at;

-- name: ListStaleStreams :many
-- A stream is "stale" when there exists at least one user-authored event newer
-- than its last_evaluated_at (or the stream has never been evaluated). The
-- agent loop polls this to discover proposals/tasks/etc. with unprocessed
-- comments. Optional target_type filter scopes the result to a single surface
-- (e.g. "proposal").
SELECT s.id, s.project_id, s.target_type, s.target_id,
       s.last_evaluated_at, s.last_evaluated_by
FROM zdx_event_streams s
WHERE s.project_id = @project_id
  AND (sqlc.narg('target_type')::text IS NULL OR s.target_type = sqlc.narg('target_type')::text)
  AND EXISTS (
    SELECT 1 FROM zdx_events e
    WHERE e.project_id = s.project_id
      AND e.target_type = s.target_type
      AND e.target_id = s.target_id
      AND e.author_kind = 'user'
      AND (s.last_evaluated_at IS NULL OR e.created_at > s.last_evaluated_at)
  )
ORDER BY s.target_type, s.target_id;

-- name: ListStaleProposalStreams :many
-- Like ListStaleStreams but with a required target_type and returns the newest
-- user event timestamp so callers can prioritize work.
SELECT s.id, s.project_id, s.target_type, s.target_id,
       s.last_evaluated_at, s.last_evaluated_by,
       (SELECT MAX(e.created_at) FROM zdx_events e
        WHERE e.project_id = s.project_id
          AND e.target_type = s.target_type
          AND e.target_id = s.target_id
          AND e.author_kind = 'user'
          AND (s.last_evaluated_at IS NULL OR e.created_at > s.last_evaluated_at)
       ) AS newest_user_event_at
FROM zdx_event_streams s
WHERE s.project_id = @project_id
  AND s.target_type = @target_type
  AND EXISTS (
    SELECT 1 FROM zdx_events e
    WHERE e.project_id = s.project_id
      AND e.target_type = s.target_type
      AND e.target_id = s.target_id
      AND e.author_kind = 'user'
      AND (s.last_evaluated_at IS NULL OR e.created_at > s.last_evaluated_at)
  )
ORDER BY newest_user_event_at DESC NULLS LAST;
