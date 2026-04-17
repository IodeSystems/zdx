-- Count resolved→open transitions for a todo key so solo can block churn.
ALTER TABLE zdx_todos ADD COLUMN reopen_count integer NOT NULL DEFAULT 0;
