-- Reverse the three TK-1759 renames. The DOWN is safe because each new
-- kind in the UP corresponds to exactly one pre-existing value; the bare
-- `triage` and `owner:triage` UP collapse is reversed by mapping back to
-- the bare form (the historically-emitted value).

UPDATE zdx_todos
   SET kind = 'owner:decompose-tracker', persona = 'owner'
 WHERE kind = 'tech:decompose-tracker';

UPDATE zdx_todos
   SET kind = 'owner:decompose-feature', persona = 'owner'
 WHERE kind = 'tech:decompose-feature';

UPDATE zdx_todos
   SET kind = 'triage', persona = 'owner'
 WHERE kind = 'product:triage';

UPDATE zdx_maturity_items
   SET kind = 'owner:decompose-feature'
 WHERE kind = 'tech:decompose-feature';
