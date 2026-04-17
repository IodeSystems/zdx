-- name: GetProjectClassification :one
SELECT classification FROM zdx_projects WHERE id = $1;

-- name: SetProjectClassification :exec
UPDATE zdx_projects SET classification = $2 WHERE id = $1;

-- name: ListDoctorDeferrals :many
SELECT id, project_id, check_name, rung, deferred_at
FROM zdx_doctor_deferrals WHERE project_id = $1
ORDER BY check_name;

-- name: DeferDoctorCheck :exec
INSERT INTO zdx_doctor_deferrals (project_id, check_name, rung)
VALUES ($1, $2, $3)
ON CONFLICT (project_id, check_name) DO UPDATE SET
  rung = EXCLUDED.rung,
  deferred_at = NOW();

-- name: UndeferDoctorCheck :exec
DELETE FROM zdx_doctor_deferrals WHERE project_id = $1 AND check_name = $2;

-- name: ClearDoctorDeferrals :exec
DELETE FROM zdx_doctor_deferrals WHERE project_id = $1;
