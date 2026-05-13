-- IS-1175: add xhigh + max complexity tier slots to admin LLM configs.
-- Nullable to match the existing low/medium/high columns; resolver
-- (IS-1176) falls through to high with a once-per-session warning when
-- a row has no value for the requested tier.

ALTER TABLE zdx_llm_configs
    ADD COLUMN model_xhigh TEXT,
    ADD COLUMN model_max   TEXT;
