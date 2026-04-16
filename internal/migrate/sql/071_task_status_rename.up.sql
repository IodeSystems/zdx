UPDATE zdx_tasks SET status = 'ready'  WHERE status = 'pending';
UPDATE zdx_tasks SET status = 'active' WHERE status = 'in_progress';
ALTER TABLE zdx_tasks ALTER COLUMN status SET DEFAULT 'wip';
