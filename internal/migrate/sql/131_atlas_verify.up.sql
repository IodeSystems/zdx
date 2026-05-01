ALTER TABLE zdx_narrative_chunks
  ADD COLUMN status TEXT NOT NULL DEFAULT 'fresh'
    CHECK (status IN ('fresh','stale','broken'));

CREATE INDEX zdx_narrative_chunks_status_idx
  ON zdx_narrative_chunks(project_id, status);
