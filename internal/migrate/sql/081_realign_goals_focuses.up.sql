-- Realign focuses (formerly themes) to match canonical goal names,
-- and link focus-blocker issues to their corresponding goals.

-- ══════════════════════════════════════════════════════════════════════════════
-- 1. Rename active focuses to match new goal names
-- ══════════════════════════════════════════════════════════════════════════════

UPDATE zdx_focuses SET name = 'Experience', description = 'One-stop software dev portal: issue/feature/task management UI, dashboards, navigation, onboarding, CLI UX'
WHERE name = 'Dev Portal';

UPDATE zdx_focuses SET name = 'Platform Operations', description = 'Reliable self-hosted platform: uptime, data integrity, auth, multi-tenancy, error handling'
WHERE name = 'Reliable Platform';

UPDATE zdx_focuses SET name = 'Planning & Maturity', description = 'Inception-to-maturity lifecycle: empty-project onboarding, goal-theme-feature-spec cascade, conformance checks'
WHERE name = 'Project Maturity';

UPDATE zdx_focuses SET name = 'Correctness', description = 'Prove code matches specs: continuous validation, regression detection, flake tracking, test-failure-to-issue auto-filing', status = 'active'
WHERE name = 'Testing';

-- ══════════════════════════════════════════════════════════════════════════════
-- 2. Link focus-blocker issues to their corresponding new goals
--    (populates zdx_goal_issues from zdx_focus_blockers + focus→goal mapping)
-- ══════════════════════════════════════════════════════════════════════════════

-- Collaborative Dev focus → Collaborative Dev goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Collaborative Dev' AND g.project_id = f.project_id
WHERE f.name = 'Collaborative Dev'
ON CONFLICT DO NOTHING;

-- Experience focus → Experience goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Experience' AND g.project_id = f.project_id
WHERE f.name = 'Experience'
ON CONFLICT DO NOTHING;

-- Platform Operations focus → Platform Operations goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Platform Operations' AND g.project_id = f.project_id
WHERE f.name = 'Platform Operations'
ON CONFLICT DO NOTHING;

-- Observability focus → Observability goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Observability' AND g.project_id = f.project_id
WHERE f.name = 'Observability'
ON CONFLICT DO NOTHING;

-- Planning & Maturity focus → Planning & Maturity goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Planning & Maturity' AND g.project_id = f.project_id
WHERE f.name = 'Planning & Maturity'
ON CONFLICT DO NOTHING;

-- Correctness focus → Correctness goal
INSERT INTO zdx_goal_issues (goal_id, issue_id)
SELECT g.id, fb.issue_id
FROM zdx_focus_blockers fb
JOIN zdx_focuses f ON f.id = fb.focus_id
JOIN zdx_project_goals g ON g.title = 'Correctness' AND g.project_id = f.project_id
WHERE f.name = 'Correctness'
ON CONFLICT DO NOTHING;
