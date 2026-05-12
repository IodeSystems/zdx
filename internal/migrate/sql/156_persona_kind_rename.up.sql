-- TK-1759: rename persona-prefixed kinds for IS-1090 contract surface.
--   owner:decompose-tracker → tech:decompose-tracker  (dev-shaped work, not metadata)
--   owner:decompose-feature → tech:decompose-feature
--   owner:triage / triage   → product:triage           (product persona owns prioritization)

UPDATE zdx_todos
   SET kind = 'tech:decompose-tracker', persona = 'tech'
 WHERE kind = 'owner:decompose-tracker';

UPDATE zdx_todos
   SET kind = 'tech:decompose-feature', persona = 'tech'
 WHERE kind = 'owner:decompose-feature';

UPDATE zdx_todos
   SET kind = 'product:triage', persona = 'product'
 WHERE kind IN ('owner:triage', 'triage');

-- Maturity items stamp their kind verbatim into queue candidates, so the
-- decompose-feature stamping kind needs to track the rename too.
UPDATE zdx_maturity_items
   SET kind = 'tech:decompose-feature'
 WHERE kind = 'owner:decompose-feature';
