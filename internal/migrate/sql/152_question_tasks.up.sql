CREATE TABLE zdx_question_tasks (
    question_id INT NOT NULL REFERENCES zdx_questions(id) ON DELETE CASCADE,
    task_id     TEXT NOT NULL REFERENCES zdx_tasks(id)     ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (question_id, task_id)
);
CREATE INDEX idx_question_tasks_question ON zdx_question_tasks (question_id);
CREATE INDEX idx_question_tasks_task ON zdx_question_tasks (task_id);
