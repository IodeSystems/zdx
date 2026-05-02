package e2e

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestDevResolveAutoBackport covers IS-825 trigger 1: when an issue is resolved
// on dev, the server auto-creates one ready backport task per active named
// version branch that has not yet received a resolution for that issue.
func TestDevResolveAutoBackport(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to seed named branches")
	}

	const slug = "e2e-dev-resolve-backport"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Dev Resolve Backport"},
		nil,
	))

	addNamedBranch := func(name string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, srv.DSN)
		if err != nil {
			t.Fatalf("pgx connect: %v", err)
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx,
			`INSERT INTO zdx_version_branches (project_id, name, role, status)
			 SELECT id, $2, 'named-release', 'active' FROM zdx_projects WHERE slug = $1`,
			slug, name); err != nil {
			t.Fatalf("insert named branch %q: %v", name, err)
		}
	}

	// Seed two active named branches before creating the issue.
	addNamedBranch("v1.0.x")
	addNamedBranch("v2.0.x")

	var issueResp struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "fix critical bug", "context": "context", "auto_ready": true},
		&issueResp,
	))
	issueRef := fmt.Sprintf("IS-%d", issueResp.ID)

	type taskRow struct {
		ID           int32  `json:"id"`
		Status       string `json:"status"`
		TargetBranch string `json:"target_branch"`
		IssueID      *int32 `json:"issue_id,omitempty"`
	}
	listTasks := func() []taskRow {
		t.Helper()
		var resp struct {
			Tasks []taskRow `json:"tasks"`
		}
		mustOK(t, apiDo(t, http.MethodGet,
			fmt.Sprintf("/api/dx/todo/issue/tasks?slug=%s&issue_id=%s", slug, issueRef),
			nil, &resp,
		))
		return resp.Tasks
	}

	// Resolve on dev — should auto-create two backport tasks.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
		map[string]any{
			"slug":   slug,
			"id":     issueResp.ID,
			"shas":   []string{"abc1234"},
			"branch": "dev",
			"source": "manual",
		}, nil,
	))

	tasks := listTasks()
	backportBranches := map[string]bool{}
	for _, task := range tasks {
		if task.TargetBranch != "dev" {
			backportBranches[task.TargetBranch] = true
			if task.Status != "ready" {
				t.Errorf("backport task for %s: want status %q got %q", task.TargetBranch, "ready", task.Status)
			}
			gotIssueRef := ""
			if task.IssueID != nil {
				gotIssueRef = fmt.Sprintf("IS-%d", *task.IssueID)
			}
			if gotIssueRef != issueRef {
				t.Errorf("backport task for %s: want issue %q got %q", task.TargetBranch, issueRef, gotIssueRef)
			}
		}
	}
	for _, branch := range []string{"v1.0.x", "v2.0.x"} {
		if !backportBranches[branch] {
			t.Errorf("expected backport task for branch %q, not found in tasks: %+v", branch, tasks)
		}
	}

	// Resolve again on dev — idempotency: no duplicate tasks.
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
		map[string]any{
			"slug":   slug,
			"id":     issueResp.ID,
			"shas":   []string{"def5678"},
			"branch": "dev",
			"source": "manual",
		}, nil,
	))
	tasksAfterResolve := listTasks()
	countFor := func(branch string) int {
		n := 0
		for _, task := range tasksAfterResolve {
			if task.TargetBranch == branch {
				n++
			}
		}
		return n
	}
	for _, branch := range []string{"v1.0.x", "v2.0.x"} {
		if n := countFor(branch); n != 1 {
			t.Errorf("idempotency: want 1 backport task for %q, got %d", branch, n)
		}
	}

	// Resolve on a named branch — must NOT generate additional backport tasks.
	tasksBefore := len(listTasks())
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
		map[string]any{
			"slug":   slug,
			"id":     issueResp.ID,
			"shas":   []string{"deadbeef"},
			"branch": "v1.0.x",
			"source": "manual",
		}, nil,
	))
	tasksAfterNamedResolve := listTasks()
	if len(tasksAfterNamedResolve) != tasksBefore {
		t.Errorf("named-branch resolve must not create new backport tasks: before=%d after=%d",
			tasksBefore, len(tasksAfterNamedResolve))
	}
}
