-- Data migration: restructure features, goals, and focuses to match the canonical model.
-- This migration is idempotent — safe to re-run.

-- ══════════════════════════════════════════════════════════════════════════════
-- 1. Retire surface-level features (CLI/UI delivery surfaces, not value drivers)
-- ══════════════════════════════════════════════════════════════════════════════

-- These features are surfaces, not demonstrable value drivers.
-- Any specs they carry should have been on the parent capability.
-- We delete them; specs cascade if any existed (they shouldn't for most).
DELETE FROM zdx_features WHERE name IN (
    'dx feature cmd',
    'dx journal cmd',
    'dx plan cmd',
    'dx theme cmd',
    'feature mgmt ui',
    'test results ui',
    'theme roadmap ui',
    'work log ui'
);

-- ══════════════════════════════════════════════════════════════════════════════
-- 2. Rename "project direction" feature to something actionable
-- ══════════════════════════════════════════════════════════════════════════════

UPDATE zdx_features SET
    name = 'project maturity guidance',
    description = 'Guide projects toward maturity across reliability, efficiency, verifiability, composability, extensibility, and maintainability within stated goals and constraints'
WHERE name = 'project direction';

-- ══════════════════════════════════════════════════════════════════════════════
-- 3. Create new goals matching the canonical model
-- ══════════════════════════════════════════════════════════════════════════════

-- Insert only if they don't exist yet (idempotent).
INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Install & Distribution',
    'Zero-config local daemon spinup, single-binary distribution, upgrade/rollback, remote mode point-and-go',
    1, 'active', 'cold_install_to_first_issue_sec', 'seconds'
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Install & Distribution');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Planning & Maturity',
    'Inception-to-maturity lifecycle: empty-project onboarding, goal-theme-feature-spec cascade, conformance checks',
    1, 'active', 'projects_with_quantified_goals_pct', 'percent'
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Planning & Maturity');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Collaborative Dev',
    'Human+LLM collaborative development: work queues, solo workflow, agent integration, session tracking',
    1, 'active', '', ''
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Collaborative Dev');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Experience',
    'One-stop software dev portal: issue/feature/task management UI, dashboards, navigation, onboarding, CLI UX',
    1, 'active', '', ''
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Experience');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Correctness',
    'Prove code matches specs: continuous validation, regression detection, flake tracking, test-failure-to-issue auto-filing',
    1, 'active', 'specs_validated_weekly_pct', 'percent'
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Correctness');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Platform Operations',
    'Reliable self-hosted platform: uptime, data integrity, auth, multi-tenancy, error handling',
    1, 'active', '', ''
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Platform Operations');

INSERT INTO zdx_project_goals (project_id, title, description, priority, status, metric_name, metric_unit)
SELECT p.id,
    'Observability',
    'Timings, counters, errors, logs, performance analysis',
    2, 'active', '', ''
FROM zdx_projects p WHERE p.slug = 'zdx'
AND NOT EXISTS (SELECT 1 FROM zdx_project_goals g WHERE g.project_id = p.id AND g.title = 'Observability');

-- Archive old goals that don't map cleanly to the new set.
UPDATE zdx_project_goals SET status = 'archived'
WHERE title IN (
    'Enable human+LLM collaborative development',
    'One-stop software dev portal',
    'Ship a reliable, self-hosted developer-experience platform',
    'Guide projects toward maturity',
    'Observability and performance',
    'Multi-branch pull request analysis'
)
AND status != 'archived';

-- ══════════════════════════════════════════════════════════════════════════════
-- 4. Attribute features to new goals
-- ══════════════════════════════════════════════════════════════════════════════

-- dx-todo, blocker-questions → Collaborative Dev
UPDATE zdx_features SET goal_id = (
    SELECT g.id FROM zdx_project_goals g
    JOIN zdx_projects p ON p.id = g.project_id
    WHERE p.slug = 'zdx' AND g.title = 'Collaborative Dev'
)
WHERE name IN ('dx-todo', 'blocker-questions')
AND goal_id IS NULL;

-- demo recordings, dx test unified, e2e test binary, feature test layers,
-- go coverage, testharness lib, vitest adapter → Correctness
UPDATE zdx_features SET goal_id = (
    SELECT g.id FROM zdx_project_goals g
    JOIN zdx_projects p ON p.id = g.project_id
    WHERE p.slug = 'zdx' AND g.title = 'Correctness'
)
WHERE name IN (
    'demo recordings', 'dx test unified', 'e2e test binary',
    'feature test layers', 'go coverage', 'testharness lib', 'vitest adapter'
)
AND goal_id IS NULL;

-- project maturity guidance → Planning & Maturity
UPDATE zdx_features SET goal_id = (
    SELECT g.id FROM zdx_project_goals g
    JOIN zdx_projects p ON p.id = g.project_id
    WHERE p.slug = 'zdx' AND g.title = 'Planning & Maturity'
)
WHERE name IN ('project maturity guidance')
AND goal_id IS NULL;

-- stable deployments → Platform Operations
UPDATE zdx_features SET goal_id = (
    SELECT g.id FROM zdx_project_goals g
    JOIN zdx_projects p ON p.id = g.project_id
    WHERE p.slug = 'zdx' AND g.title = 'Platform Operations'
)
WHERE name IN ('stable deployments')
AND goal_id IS NULL;

-- ══════════════════════════════════════════════════════════════════════════════
-- 5. Set feature kind — testharness lib is a multiplier
-- ══════════════════════════════════════════════════════════════════════════════

UPDATE zdx_features SET kind = 'multiplier'
WHERE name = 'testharness lib' AND kind = 'direct';

-- ══════════════════════════════════════════════════════════════════════════════
-- 6. Update component names to match goals (component = theme = goal area)
-- ══════════════════════════════════════════════════════════════════════════════

UPDATE zdx_features SET component = 'CollaborativeDev'
WHERE component = '' AND name IN ('dx-todo', 'blocker-questions');

UPDATE zdx_features SET component = 'Correctness'
WHERE component = 'DxConformance';

UPDATE zdx_features SET component = 'PlanningMaturity'
WHERE component = 'ProjectDirection';

UPDATE zdx_features SET component = 'PlatformOps'
WHERE component = 'StableDeployments';
