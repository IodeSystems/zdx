-- name: UpsertTaskReview :one
INSERT INTO zdx_task_reviews (project_id, task_id, reviewer_role, reviewer_user_id, verdict, body)
VALUES (@project_id, @task_id, @reviewer_role, @reviewer_user_id, @verdict, @body)
ON CONFLICT (project_id, task_id, reviewer_role) DO UPDATE
    SET verdict          = EXCLUDED.verdict,
        body             = EXCLUDED.body,
        reviewer_user_id = EXCLUDED.reviewer_user_id,
        updated_at       = NOW()
RETURNING id, project_id, task_id, reviewer_role, reviewer_user_id, verdict, body, created_at, updated_at;

-- name: ListTaskReviews :many
SELECT r.id, r.project_id, r.task_id, r.reviewer_role, r.reviewer_user_id, r.verdict, r.body, r.created_at, r.updated_at,
       u.email AS reviewer_email
FROM zdx_task_reviews r
LEFT JOIN zdx_users u ON u.id = r.reviewer_user_id
WHERE r.project_id = $1 AND r.task_id = $2
ORDER BY r.created_at ASC;

-- name: GetTaskReview :one
SELECT id, project_id, task_id, reviewer_role, reviewer_user_id, verdict, body, created_at, updated_at
FROM zdx_task_reviews
WHERE id = $1;

-- name: GetTaskReviewByTask :one
SELECT id, project_id, task_id, reviewer_role, reviewer_user_id, verdict, body, created_at, updated_at
FROM zdx_task_reviews
WHERE project_id = $1 AND task_id = $2 AND reviewer_role = $3;
