ALTER TABLE zdx_specs RENAME COLUMN importance TO kind;
ALTER TABLE zdx_specs ADD COLUMN concern_type text NOT NULL DEFAULT 'functional';
