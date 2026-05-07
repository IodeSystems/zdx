ALTER TABLE zdx_questions ADD COLUMN owner_user_id INT REFERENCES zdx_users(id) ON DELETE SET NULL;
CREATE INDEX idx_questions_owner_unanswered ON zdx_questions(owner_user_id) WHERE owner_user_id IS NOT NULL AND answer IS NULL;
