ALTER TABLE zdx_tasks ALTER COLUMN status SET DEFAULT 'pending';
UPDATE zdx_tasks SET status = 'pending'     WHERE status = 'ready';
UPDATE zdx_tasks SET status = 'in_progress' WHERE status = 'active';
