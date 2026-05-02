ALTER TABLE zdx_events ADD COLUMN addressing_event_id BIGINT REFERENCES zdx_events(id);
CREATE INDEX zdx_events_addressing_idx ON zdx_events(addressing_event_id) WHERE addressing_event_id IS NOT NULL;
