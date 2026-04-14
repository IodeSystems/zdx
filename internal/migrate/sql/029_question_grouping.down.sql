DROP INDEX IF EXISTS idx_questions_parent;
ALTER TABLE zdx_questions DROP COLUMN IF EXISTS parent_question_id;
