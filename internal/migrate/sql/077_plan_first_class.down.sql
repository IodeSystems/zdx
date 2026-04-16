DROP TABLE IF EXISTS zdx_plan_step_refs;
DROP TABLE IF EXISTS zdx_plan_steps;

ALTER TABLE zdx_plans ALTER COLUMN feature_id SET NOT NULL;
ALTER TABLE zdx_plans ADD CONSTRAINT zdx_plans_feature_id_key UNIQUE (feature_id);

ALTER TABLE zdx_plans DROP CONSTRAINT IF EXISTS zdx_plans_project_id_fkey;
ALTER TABLE zdx_plans DROP COLUMN updated_at;
ALTER TABLE zdx_plans DROP COLUMN created_at;
ALTER TABLE zdx_plans DROP COLUMN issue_id;
ALTER TABLE zdx_plans DROP COLUMN focus_id;
ALTER TABLE zdx_plans DROP COLUMN body;
ALTER TABLE zdx_plans DROP COLUMN title;
ALTER TABLE zdx_plans DROP COLUMN project_id;
