-- Revert focus renames
UPDATE zdx_focuses SET name = 'Dev Portal' WHERE name = 'Experience';
UPDATE zdx_focuses SET name = 'Reliable Platform' WHERE name = 'Platform Operations';
UPDATE zdx_focuses SET name = 'Project Maturity' WHERE name = 'Planning & Maturity';
UPDATE zdx_focuses SET name = 'Testing', status = 'done' WHERE name = 'Correctness';

-- Remove goal→issue links added by the migration
DELETE FROM zdx_goal_issues WHERE goal_id IN (
    SELECT id FROM zdx_project_goals WHERE title IN (
        'Collaborative Dev', 'Experience', 'Platform Operations',
        'Observability', 'Planning & Maturity', 'Correctness'
    )
);
