-- Integration tokens become more flexible:
--
--   1. project_id may be NULL — an "unbound" token authorizes ingest for any
--      project the request body names. Useful for cross-project shared-secret
--      emitters. Bound tokens (today's default) still work unchanged.
--   2. capabilities is an explicit text[] of permission strings. Today it's
--      always {ingest:logs}; future expansion (ingest:metrics, ingest:traces)
--      is a value addition rather than a schema change.
--
-- Existing rows keep project_id and gain the default capability set.

ALTER TABLE zdx_integration_token
    ALTER COLUMN project_id DROP NOT NULL;

ALTER TABLE zdx_integration_token
    ADD COLUMN capabilities text[] NOT NULL DEFAULT ARRAY['ingest:logs']::text[];
