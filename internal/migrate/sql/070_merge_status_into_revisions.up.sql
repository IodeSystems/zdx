INSERT INTO zdx_revisions (project_id, target_type, target_id, field, old_val, new_val, agent, session_id, user_id, created_at)
SELECT project_id, target_type, target_id, 'status', from_status, to_status, agent_id, session_id, user_id, created_at
FROM zdx_status_events;

DROP TABLE zdx_status_events;
