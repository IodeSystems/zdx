DELETE FROM zdx_todo_incomplete_reports
 WHERE agent_id = 'backfill'
   AND reason = 'capability_gap'
   AND evidence->>'missing_capability' = 'mark-comment-as-read';
