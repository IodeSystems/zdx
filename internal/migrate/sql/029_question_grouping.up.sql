-- Add parent_question_id for question grouping (sibling/variation relationships).
ALTER TABLE zdx_questions ADD COLUMN parent_question_id INT REFERENCES zdx_questions(id) ON DELETE SET NULL;
CREATE INDEX idx_questions_parent ON zdx_questions (parent_question_id) WHERE parent_question_id IS NOT NULL;
