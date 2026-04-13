-- Track when a feature was last reviewed by the owner.
ALTER TABLE zdx_features ADD COLUMN last_reviewed_at TIMESTAMPTZ;
