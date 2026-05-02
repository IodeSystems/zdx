DROP INDEX IF EXISTS zdx_events_addressing_idx;
ALTER TABLE zdx_events DROP COLUMN IF EXISTS addressing_event_id;
