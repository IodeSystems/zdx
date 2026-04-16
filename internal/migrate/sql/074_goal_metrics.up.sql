-- Goals can carry a measured metric (nullable by convention — empty string = not yet quantified).
ALTER TABLE zdx_project_goals ADD COLUMN metric_name text NOT NULL DEFAULT '';
ALTER TABLE zdx_project_goals ADD COLUMN metric_unit text NOT NULL DEFAULT '';
