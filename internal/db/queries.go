package db

import (
	"context"
	"fmt"
)

// This file mirrors what `sqlc generate` produces from queries/dx.sql.
// Regenerate with: make sqlgen

// ── projects ──────────────────────────────────────────────────────────────────

func (q *Queries) ListProjects(ctx context.Context) ([]Project, error) {
	rows, err := q.db.Query(ctx, `SELECT id, slug, name, created_at FROM zdx_projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Slug, &p.Name, &p.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (q *Queries) GetProjectBySlug(ctx context.Context, slug string) (Project, error) {
	var p Project
	err := q.db.QueryRow(ctx,
		`SELECT id, slug, name, created_at FROM zdx_projects WHERE slug = $1`, slug,
	).Scan(&p.ID, &p.Slug, &p.Name, &p.CreatedAt)
	return p, err
}

func (q *Queries) CreateProject(ctx context.Context, slug, name string) (Project, error) {
	var p Project
	err := q.db.QueryRow(ctx,
		`INSERT INTO zdx_projects (slug, name) VALUES ($1, $2) RETURNING id, slug, name, created_at`,
		slug, name,
	).Scan(&p.ID, &p.Slug, &p.Name, &p.CreatedAt)
	return p, err
}

// nextID atomically increments and returns the next IS-N or TK-N counter.
func (q *Queries) nextID(ctx context.Context, projectID int32, kind string) (int32, error) {
	var val int32
	err := q.db.QueryRow(ctx, `
		INSERT INTO zdx_id_seq (project_id, kind, next_val) VALUES ($1, $2, 2)
		ON CONFLICT (project_id, kind) DO UPDATE SET next_val = zdx_id_seq.next_val + 1
		RETURNING next_val - 1`,
		projectID, kind,
	).Scan(&val)
	return val, err
}

func (q *Queries) NextIssueID(ctx context.Context, projectID int32) (string, error) {
	n, err := q.nextID(ctx, projectID, "issue")
	return fmt.Sprintf("IS-%d", n), err
}

func (q *Queries) NextTaskID(ctx context.Context, projectID int32) (string, error) {
	n, err := q.nextID(ctx, projectID, "task")
	return fmt.Sprintf("TK-%d", n), err
}

// ── issues ────────────────────────────────────────────────────────────────────

const issueColumns = `id, project_id, title, status, priority, component, context, blocked_by, created_at`

func scanIssue(row interface{ Scan(...any) error }, i *Issue) error {
	return row.Scan(&i.ID, &i.ProjectID, &i.Title, &i.Status, &i.Priority, &i.Component, &i.Context, &i.BlockedBy, &i.CreatedAt)
}

func (q *Queries) ListIssues(ctx context.Context, projectID int32) ([]Issue, error) {
	rows, err := q.db.Query(ctx,
		`SELECT `+issueColumns+` FROM zdx_issues WHERE project_id = $1 ORDER BY priority NULLS LAST, created_at`,
		projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Issue
	for rows.Next() {
		var i Issue
		if err := scanIssue(rows, &i); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, nil
}

func (q *Queries) GetIssue(ctx context.Context, projectID int32, id string) (Issue, error) {
	var i Issue
	err := scanIssue(q.db.QueryRow(ctx,
		`SELECT `+issueColumns+` FROM zdx_issues WHERE project_id = $1 AND id = $2`,
		projectID, id), &i)
	return i, err
}

type CreateIssueParams struct {
	ID        string
	ProjectID int32
	Title     string
	Context   string
	Priority  string
	Component string
}

func (q *Queries) CreateIssue(ctx context.Context, p CreateIssueParams) (Issue, error) {
	var i Issue
	err := scanIssue(q.db.QueryRow(ctx, `
		INSERT INTO zdx_issues (id, project_id, title, context, priority, component)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING `+issueColumns,
		p.ID, p.ProjectID, p.Title, p.Context, p.Priority, p.Component), &i)
	return i, err
}

type UpdateIssueParams struct {
	ProjectID int32
	ID        string
	Title     string
	Context   string
	Priority  string
	BlockedBy string
}

func (q *Queries) UpdateIssue(ctx context.Context, p UpdateIssueParams) error {
	_, err := q.db.Exec(ctx, `
		UPDATE zdx_issues
		SET title      = COALESCE(NULLIF($3, ''),  title),
		    context    = COALESCE(NULLIF($4, ''),  context),
		    priority   = COALESCE(NULLIF($5, ''),  priority),
		    blocked_by = COALESCE(NULLIF($6, ''),  blocked_by)
		WHERE project_id = $1 AND id = $2`,
		p.ProjectID, p.ID, p.Title, p.Context, p.Priority, p.BlockedBy)
	return err
}

func (q *Queries) CloseIssue(ctx context.Context, projectID int32, id string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE zdx_issues SET status = 'closed' WHERE project_id = $1 AND id = $2`,
		projectID, id)
	return err
}

func (q *Queries) AppendIssueWork(ctx context.Context, issueID, agent, note string) error {
	_, err := q.db.Exec(ctx,
		`INSERT INTO zdx_issue_work (issue_id, agent, note) VALUES ($1, $2, $3)`,
		issueID, agent, note)
	return err
}

func (q *Queries) GetIssueWork(ctx context.Context, issueID string) ([]IssueWork, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, issue_id, agent, note, created_at FROM zdx_issue_work WHERE issue_id = $1 ORDER BY created_at`,
		issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IssueWork
	for rows.Next() {
		var w IssueWork
		if err := rows.Scan(&w.ID, &w.IssueID, &w.Agent, &w.Note, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, nil
}

// ── tasks ─────────────────────────────────────────────────────────────────────

const taskColumns = `id, project_id, text, feature, status, reason, issue, depends, test_plan, test_refs, created_at, completed_at`

func scanTask(row interface{ Scan(...any) error }, t *Task) error {
	return row.Scan(&t.ID, &t.ProjectID, &t.Text, &t.Feature, &t.Status, &t.Reason, &t.Issue, &t.Depends, &t.TestPlan, &t.TestRefs, &t.CreatedAt, &t.CompletedAt)
}

func (q *Queries) ListTasks(ctx context.Context, projectID int32) ([]Task, error) {
	rows, err := q.db.Query(ctx,
		`SELECT `+taskColumns+` FROM zdx_tasks WHERE project_id = $1 ORDER BY created_at`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (q *Queries) ListTasksByFeature(ctx context.Context, projectID int32, feature string) ([]Task, error) {
	rows, err := q.db.Query(ctx,
		`SELECT `+taskColumns+` FROM zdx_tasks WHERE project_id = $1 AND feature = $2 ORDER BY created_at`,
		projectID, feature)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

func (q *Queries) ListTasksByIssue(ctx context.Context, projectID int32, issueID string) ([]Task, error) {
	rows, err := q.db.Query(ctx,
		`SELECT `+taskColumns+` FROM zdx_tasks WHERE project_id = $1 AND issue = $2 ORDER BY created_at`,
		projectID, issueID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Task
	for rows.Next() {
		var t Task
		if err := scanTask(rows, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

type CreateTaskParams struct {
	ID        string
	ProjectID int32
	Text      string
	Feature   string
	Issue     string
}

func (q *Queries) CreateTask(ctx context.Context, p CreateTaskParams) (Task, error) {
	var t Task
	err := scanTask(q.db.QueryRow(ctx, `
		INSERT INTO zdx_tasks (id, project_id, text, feature, issue)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING `+taskColumns,
		p.ID, p.ProjectID, p.Text, p.Feature, p.Issue), &t)
	return t, err
}

func (q *Queries) UpdateTaskStatus(ctx context.Context, id, status, reason string) error {
	_, err := q.db.Exec(ctx, `UPDATE zdx_tasks SET status = $2, reason = $3 WHERE id = $1`, id, status, reason)
	return err
}

func (q *Queries) MarkTaskDone(ctx context.Context, id, testPlan, testRefs string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE zdx_tasks SET status = 'done', test_plan = $2, test_refs = $3, completed_at = NOW() WHERE id = $1`,
		id, testPlan, testRefs)
	return err
}

func (q *Queries) MarkTaskUndone(ctx context.Context, id string) error {
	_, err := q.db.Exec(ctx,
		`UPDATE zdx_tasks SET status = 'pending', completed_at = NULL WHERE id = $1`, id)
	return err
}

// ── features ──────────────────────────────────────────────────────────────────

const featureColumns = `id, project_id, name, description, what, why, done_when, component`

func scanFeature(row interface{ Scan(...any) error }, f *Feature) error {
	return row.Scan(&f.ID, &f.ProjectID, &f.Name, &f.Description, &f.What, &f.Why, &f.DoneWhen, &f.Component)
}

func (q *Queries) ListFeatures(ctx context.Context, projectID int32) ([]Feature, error) {
	rows, err := q.db.Query(ctx,
		`SELECT `+featureColumns+` FROM zdx_features WHERE project_id = $1 ORDER BY name`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Feature
	for rows.Next() {
		var f Feature
		if err := scanFeature(rows, &f); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

func (q *Queries) GetFeature(ctx context.Context, projectID int32, name string) (Feature, error) {
	var f Feature
	err := scanFeature(q.db.QueryRow(ctx,
		`SELECT `+featureColumns+` FROM zdx_features WHERE project_id = $1 AND name = $2`,
		projectID, name), &f)
	return f, err
}

func (q *Queries) UpsertFeature(ctx context.Context, projectID int32, name, description string) (Feature, error) {
	var f Feature
	err := scanFeature(q.db.QueryRow(ctx, `
		INSERT INTO zdx_features (project_id, name, description)
		VALUES ($1, $2, $3)
		ON CONFLICT (project_id, name) DO UPDATE SET description = EXCLUDED.description
		RETURNING `+featureColumns, projectID, name, description), &f)
	return f, err
}

type UpdateFeatureFieldParams struct {
	ProjectID int32
	Name      string
	Field     string
	Value     string
}

func (q *Queries) UpdateFeatureField(ctx context.Context, p UpdateFeatureFieldParams) error {
	_, err := q.db.Exec(ctx, `
		UPDATE zdx_features
		SET description = CASE WHEN $3 = 'description' THEN $4 ELSE description END,
		    what        = CASE WHEN $3 = 'what'        THEN $4 ELSE what        END,
		    why         = CASE WHEN $3 = 'why'         THEN $4 ELSE why         END,
		    done_when   = CASE WHEN $3 = 'done_when'   THEN $4 ELSE done_when   END,
		    component   = CASE WHEN $3 = 'component'   THEN $4 ELSE component   END
		WHERE project_id = $1 AND name = $2`,
		p.ProjectID, p.Name, p.Field, p.Value)
	return err
}

func (q *Queries) ListSpecs(ctx context.Context, featureID int32) ([]Spec, error) {
	rows, err := q.db.Query(ctx,
		`SELECT id, feature_id, description, kind FROM zdx_specs WHERE feature_id = $1 ORDER BY id`, featureID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Spec
	for rows.Next() {
		var s Spec
		if err := rows.Scan(&s.ID, &s.FeatureID, &s.Description, &s.Kind); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, nil
}

func (q *Queries) AddSpec(ctx context.Context, featureID int32, description, kind string) (Spec, error) {
	var s Spec
	err := q.db.QueryRow(ctx,
		`INSERT INTO zdx_specs (feature_id, description, kind) VALUES ($1, $2, $3) RETURNING id, feature_id, description, kind`,
		featureID, description, kind,
	).Scan(&s.ID, &s.FeatureID, &s.Description, &s.Kind)
	return s, err
}
