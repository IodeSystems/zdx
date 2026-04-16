-- Data backfill is not cleanly reversible — deleted features and their specs are gone.
-- This down migration restores goal/feature states to approximate pre-migration values.

-- Unarchive old goals
UPDATE zdx_project_goals SET status = 'active'
WHERE title IN (
    'Enable human+LLM collaborative development',
    'One-stop software dev portal',
    'Ship a reliable, self-hosted developer-experience platform',
    'Guide projects toward maturity',
    'Observability and performance'
);

-- Remove new goals
DELETE FROM zdx_project_goals WHERE title IN (
    'Install & Distribution',
    'Planning & Maturity',
    'Collaborative Dev',
    'Experience',
    'Correctness',
    'Platform Operations',
    'Observability'
);

-- Clear goal attributions
UPDATE zdx_features SET goal_id = NULL;
UPDATE zdx_features SET kind = 'direct' WHERE kind = 'multiplier';

-- Restore component names
UPDATE zdx_features SET component = 'DxConformance' WHERE component = 'Correctness';
UPDATE zdx_features SET component = 'ProjectDirection' WHERE component = 'PlanningMaturity';
UPDATE zdx_features SET component = 'StableDeployments' WHERE component = 'PlatformOps';

-- Restore renamed feature
UPDATE zdx_features SET name = 'project direction'
WHERE name = 'project maturity guidance';
