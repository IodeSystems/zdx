DROP TABLE IF EXISTS zdx_feature_multipliers;

ALTER TABLE zdx_features DROP COLUMN graph_url;
ALTER TABLE zdx_features DROP COLUMN target_value;
ALTER TABLE zdx_features DROP COLUMN baseline_value;
ALTER TABLE zdx_features DROP COLUMN metric_unit;
ALTER TABLE zdx_features DROP COLUMN metric_name;
ALTER TABLE zdx_features DROP COLUMN goal_id;
ALTER TABLE zdx_features DROP COLUMN kind;
ALTER TABLE zdx_features DROP COLUMN parent_feature_id;
