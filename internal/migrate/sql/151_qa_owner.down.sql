DROP INDEX IF EXISTS idx_questions_owner_unanswered;
ALTER TABLE zdx_questions DROP COLUMN IF EXISTS owner_user_id;
