-- Specs carry a concern type beyond must/should/nice-to-have.
-- Default 'functional'; other values: latency, security, ux, compatibility, etc.
ALTER TABLE zdx_specs ADD COLUMN concern_type text NOT NULL DEFAULT 'functional';
