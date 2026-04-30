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
