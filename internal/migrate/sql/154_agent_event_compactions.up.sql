-- Attached compactions for agent session events.
--
-- Original events live in the per-session JSONL log under .zdx/agent/openai/
-- and are INSERT-only. Compactions are ATTACHED here as separate rows so
-- multiple strategies can produce different compacted forms for the same
-- event without ever rewriting the original. Hydration (rebuilding the chat
-- msgs[] for a session) picks the most recent compaction per event_id whose
-- strategy is in the active hydration set.
--
-- event_id intentionally has no FK: the events table does not yet exist in
-- Postgres (event log lives in JSONL). When/if events move to Postgres the
-- FK can be added in a follow-up migration.
CREATE TABLE zdx_agent_event_compactions (
    id BIGSERIAL PRIMARY KEY,
    event_id TEXT NOT NULL,
    strategy TEXT NOT NULL,
    compacted_content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_agent_event_compactions_event_id
    ON zdx_agent_event_compactions (event_id, created_at DESC);
