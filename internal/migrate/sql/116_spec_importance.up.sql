-- Rename specs.kind → specs.importance (value-loss tier: must/should/nice-to-have)
ALTER TABLE zdx_specs RENAME COLUMN kind TO importance;

-- Drop concern_type — organizational concerns handled by feature decomposition
ALTER TABLE zdx_specs DROP COLUMN concern_type;
