CREATE TABLE zdx_goal_issues (
    goal_id   INTEGER NOT NULL REFERENCES zdx_project_goals(id) ON DELETE CASCADE,
    issue_id  TEXT    NOT NULL REFERENCES zdx_issues(id) ON DELETE CASCADE,
    PRIMARY KEY (goal_id, issue_id)
);
CREATE INDEX idx_goal_issues_issue ON zdx_goal_issues (issue_id);
