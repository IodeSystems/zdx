-- Upgrade plans from feature-only to first-class living objects.
-- Plans can anchor to a focus, feature, or issue (or standalone).

-- Add new columns.
ALTER TABLE zdx_plans ADD COLUMN project_id integer;
ALTER TABLE zdx_plans ADD COLUMN title text NOT NULL DEFAULT '';
ALTER TABLE zdx_plans ADD COLUMN body text NOT NULL DEFAULT '';
ALTER TABLE zdx_plans ADD COLUMN focus_id integer REFERENCES zdx_focuses(id);
ALTER TABLE zdx_plans ADD COLUMN issue_id text;
ALTER TABLE zdx_plans ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
ALTER TABLE zdx_plans ADD COLUMN updated_at timestamptz NOT NULL DEFAULT now();

-- Backfill project_id from feature.
UPDATE zdx_plans p SET project_id = f.project_id
FROM zdx_features f WHERE f.id = p.feature_id;

-- For any orphaned plans (shouldn't exist, but defensive).
DELETE FROM zdx_plans WHERE project_id IS NULL;
ALTER TABLE zdx_plans ALTER COLUMN project_id SET NOT NULL;
ALTER TABLE zdx_plans ADD CONSTRAINT zdx_plans_project_id_fkey
    FOREIGN KEY (project_id) REFERENCES zdx_projects(id);

-- Drop the 1:1 unique constraint on feature_id — plans are now M:N capable.
ALTER TABLE zdx_plans DROP CONSTRAINT zdx_plans_feature_id_key;

-- Make feature_id nullable (plans can anchor to focus or issue instead).
ALTER TABLE zdx_plans ALTER COLUMN feature_id DROP NOT NULL;

-- Plan steps: ordered children of a plan.
CREATE TABLE zdx_plan_steps (
    id serial PRIMARY KEY,
    plan_id integer NOT NULL REFERENCES zdx_plans(id) ON DELETE CASCADE,
    seq integer NOT NULL DEFAULT 0,
    text text NOT NULL DEFAULT '',
    status text NOT NULL DEFAULT 'pending',
    depends_on integer REFERENCES zdx_plan_steps(id),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Step → spawned refs (issues, features, tasks created from discovery).
CREATE TABLE zdx_plan_step_refs (
    step_id integer NOT NULL REFERENCES zdx_plan_steps(id) ON DELETE CASCADE,
    target_type text NOT NULL,
    target_id text NOT NULL,
    PRIMARY KEY (step_id, target_type, target_id)
);
