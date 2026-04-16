-- Feature tree: parent link for decomposition.
ALTER TABLE zdx_features ADD COLUMN parent_feature_id integer REFERENCES zdx_features(id);

-- Feature kind: 'direct' (deposits goal currency) or 'multiplier' (amplifies other features).
ALTER TABLE zdx_features ADD COLUMN kind text NOT NULL DEFAULT 'direct';

-- Goal attribution: which goal does this feature serve?
ALTER TABLE zdx_features ADD COLUMN goal_id integer REFERENCES zdx_project_goals(id);

-- Multiplier metrics (required for multiplier readiness gate, nullable for direct).
ALTER TABLE zdx_features ADD COLUMN metric_name text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN metric_unit text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN baseline_value text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN target_value text NOT NULL DEFAULT '';
ALTER TABLE zdx_features ADD COLUMN graph_url text NOT NULL DEFAULT '';

-- Multiplier links: which features does a multiplier feature amplify?
CREATE TABLE zdx_feature_multipliers (
    feature_id integer NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    multiplies_feature_id integer NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    PRIMARY KEY (feature_id, multiplies_feature_id),
    CHECK (feature_id != multiplies_feature_id)
);
