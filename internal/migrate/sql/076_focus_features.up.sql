-- M:N join: any number of features can be attributed to any number of focuses.
CREATE TABLE zdx_focus_features (
    focus_id integer NOT NULL REFERENCES zdx_focuses(id) ON DELETE CASCADE,
    feature_id integer NOT NULL REFERENCES zdx_features(id) ON DELETE CASCADE,
    PRIMARY KEY (focus_id, feature_id)
);
