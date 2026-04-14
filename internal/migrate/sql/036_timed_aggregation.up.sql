ALTER TABLE zdx_timed ADD COLUMN count INT NOT NULL DEFAULT 1;
ALTER TABLE zdx_timed ADD COLUMN total_ms BIGINT NOT NULL DEFAULT 0;

-- Backfill: set total_ms = duration_ms for existing rows
UPDATE zdx_timed SET total_ms = duration_ms WHERE total_ms = 0;
