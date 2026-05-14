ALTER TABLE zdx_todos
    DROP COLUMN IF EXISTS claim_predicate_snapshot,
    DROP COLUMN IF EXISTS source;
