package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

// TestDevResolveBackportSyncClean covers the IS-825 trigger 1 sync-clean
// gate: when the project has a git repo and the target named branch is an
// ancestor of dev (i.e. routine dev→target ff-merge will land the
// resolution naturally), no backport task is created. When target has
// diverged from dev, the task is created as before.
//
// This is the inverse-of-spurious test for the long-standing complaint
// that backport tasks fire unconditionally on every dev resolution.
func TestDevResolveBackportSyncClean(t *testing.T) {
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to seed named branches")
	}

	runCase := func(t *testing.T, slug string, divergeTarget bool, wantTaskForTarget bool, targetBranch string) {
		t.Helper()
		// Clear any clone from a prior run so EnsureRepo creates a fresh
		// checkout pointing at this run's source repo.
		for _, base := range []string{"repos", "../e2e/repos", filepath.Join(os.Getenv("ZDX_HOME"), "data/repos")} {
			if base == "" {
				continue
			}
			_ = os.RemoveAll(filepath.Join(base, slug))
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/project",
			map[string]string{"slug": slug, "name": "E2E Backport Sync " + slug}, nil))

		// Build a local source repo with two branches.
		src := t.TempDir()
		git := func(args ...string) string {
			t.Helper()
			cmd := exec.Command("git", append([]string{"-C", src}, args...)...)
			cmd.Env = append(cmd.Env,
				"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e2e",
				"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e2e",
				"HOME="+t.TempDir(), "PATH=/usr/bin:/bin",
				"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
			return strings.TrimSpace(string(out))
		}
		write := func(name, body string) {
			t.Helper()
			if err := exec.Command("sh", "-c", "echo '"+body+"' > "+filepath.Join(src, name)).Run(); err != nil {
				t.Fatalf("write: %v", err)
			}
		}
		git("init", "-q", "-b", targetBranch)
		git("config", "user.email", "t@e2e")
		git("config", "user.name", "t")
		write("README", "init")
		git("add", ".")
		git("commit", "-q", "-m", "init")
		// Branch dev off targetBranch and add a dev-only commit (the
		// "resolution"). targetBranch remains an ancestor of dev in the
		// clean case.
		git("checkout", "-q", "-b", "dev")
		write("dev-only", "feature work")
		git("add", ".")
		git("commit", "-q", "-m", "feature on dev")
		if divergeTarget {
			// Add a target-only commit so dev cannot ff-merge into target.
			git("checkout", "-q", targetBranch)
			write("target-only", "hotfix on target")
			git("add", ".")
			git("commit", "-q", "-m", "hotfix on "+targetBranch)
		}
		git("checkout", "-q", "dev")

		// Configure project git. git_branch is the primary single-branch
		// clone; EnsureBranchFetched widens to grab the other branch.
		mustOK(t, apiDo(t, http.MethodPut, "/api/admin/project-git-config",
			map[string]any{"slug": slug, "git_url": "file://" + src, "git_branch": "dev"}, nil))

		// Seed the named release branch row so the trigger has a target.
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
			slug, targetBranch); err != nil {
			t.Fatalf("insert named branch: %v", err)
		}

		var issueResp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{"slug": slug, "title": "sync-gate test", "context": "ctx", "auto_ready": true},
			&issueResp,
		))
		issueRef := fmt.Sprintf("IS-%d", issueResp.ID)

		// Resolve on dev — trigger 1 fires.
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{
				"slug": slug, "id": issueResp.ID,
				"shas": []string{"abc1234"}, "branch": "dev", "source": "manual",
			}, nil,
		))

		type taskRow struct {
			ID           int32  `json:"id"`
			Status       string `json:"status"`
			TargetBranch string `json:"target_branch"`
		}
		var resp struct {
			Tasks []taskRow `json:"tasks"`
		}
		mustOK(t, apiDo(t, http.MethodGet,
			fmt.Sprintf("/api/dx/todo/issue/tasks?slug=%s&issue_id=%s", slug, issueRef),
			nil, &resp,
		))
		hasTarget := false
		for _, task := range resp.Tasks {
			if task.TargetBranch == targetBranch {
				hasTarget = true
			}
		}
		if hasTarget != wantTaskForTarget {
			t.Errorf("backport task for %q: want=%v got=%v (tasks=%+v)",
				targetBranch, wantTaskForTarget, hasTarget, resp.Tasks)
		}

		// Sync-todo assertion: clean sync → one open `branch-sync` todo
		// keyed by (source → target). Diverged sync → no such todo (the
		// backport task carries the operator instruction instead).
		var todoCheck struct {
			Key    string `json:"key"`
			Status string `json:"status"`
		}
		wantSyncTodo := !divergeTarget
		expectedKey := fmt.Sprintf("branch-sync:dev:%s", targetBranch)
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		conn2, err := pgx.Connect(ctx2, srv.DSN)
		if err != nil {
			t.Fatalf("pgx connect (sync todo check): %v", err)
		}
		defer conn2.Close(ctx2)
		err = conn2.QueryRow(ctx2,
			`SELECT key, status FROM zdx_todos
			 WHERE project_id = (SELECT id FROM zdx_projects WHERE slug = $1)
			   AND key = $2`,
			slug, expectedKey).Scan(&todoCheck.Key, &todoCheck.Status)
		gotSyncTodo := err == nil && todoCheck.Key == expectedKey
		if gotSyncTodo != wantSyncTodo {
			t.Errorf("branch-sync todo for %q: want=%v got=%v (err=%v key=%q)",
				expectedKey, wantSyncTodo, gotSyncTodo, err, todoCheck.Key)
		}
	}

	t.Run("clean_sync_skips_backport_task", func(t *testing.T) {
		runCase(t, "e2e-backport-sync-clean", false, false, "main")
	})
	t.Run("diverged_target_creates_backport_task", func(t *testing.T) {
		runCase(t, "e2e-backport-sync-diverged", true, true, "main")
	})
}
