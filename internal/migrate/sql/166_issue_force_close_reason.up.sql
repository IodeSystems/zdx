-- IS-1088: distinguish force-bypass closes from categorical/clean closes.
-- force_close_reason captures the operator's bypass justification
-- (heuristic-false-positive, emergency, rollback, other, or any free-form
-- string). Null for pre-IS-1088 closes and for categorical reasons
-- (duplicate, wontfix, superseded, link) which keep their existing marker.
ALTER TABLE zdx_issues ADD COLUMN force_close_reason text;
CREATE INDEX idx_zdx_issues_force_close_reason ON zdx_issues (project_id) WHERE force_close_reason IS NOT NULL;
