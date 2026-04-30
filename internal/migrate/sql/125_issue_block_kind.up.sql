-- Distinguish composition (tracker → child) from sequencing (real "X waits for Y") edges.
-- All existing edges default to 'sequencing'; new edges from --parent flag will write 'composition'.
-- Backfill of existing tracker→child edges is deferred to a manual / one-shot re-tag pass.
ALTER TABLE zdx_issue_blocks
  ADD COLUMN kind text NOT NULL DEFAULT 'sequencing';

CREATE INDEX idx_issue_blocks_kind ON zdx_issue_blocks(kind);
