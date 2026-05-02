-- name: InsertSideEffectIfNew :one
INSERT INTO zdx_incomplete_report_side_effects (project_id, reason, evidence_fingerprint, action_type, meta)
VALUES (@project_id, @reason, @evidence_fingerprint, @action_type, @meta)
ON CONFLICT (project_id, reason, evidence_fingerprint, action_type) DO NOTHING
RETURNING id, project_id, reason, evidence_fingerprint, action_type, meta, fired_at;

-- name: ListSideEffects :many
SELECT id, project_id, reason, evidence_fingerprint, action_type, meta, fired_at
FROM zdx_incomplete_report_side_effects
WHERE project_id = @project_id
ORDER BY fired_at DESC;
