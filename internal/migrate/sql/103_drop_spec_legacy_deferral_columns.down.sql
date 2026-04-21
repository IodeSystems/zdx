ALTER TABLE zdx_specs ADD COLUMN deferred boolean NOT NULL DEFAULT false;
ALTER TABLE zdx_specs ADD COLUMN deferred_reason text NOT NULL DEFAULT '';
