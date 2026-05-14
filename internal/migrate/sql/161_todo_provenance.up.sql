-- IS-991 (absorbing the cascade-closed IS-990 schema slice): add provenance
-- columns to zdx_todos so cycle detection and the operator can answer "which
-- query produced this candidate, and what did it consult?"
--
-- source: structured "QueryName(k=v,k=v)" string stamped by the emitting
--   branch of generateAgentQueue. Empty for legacy rows and for non-candidate
--   writers (cycle-detection-only paths).
-- claim_predicate_snapshot: jsonb dictionary captured at claim time (IS-992
--   wires the write; this issue only adds the column so the surface is ready).
--
-- Both columns are NOT NULL with defaults so existing rows migrate trivially.

ALTER TABLE zdx_todos
    ADD COLUMN source TEXT NOT NULL DEFAULT '',
    ADD COLUMN claim_predicate_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb;
