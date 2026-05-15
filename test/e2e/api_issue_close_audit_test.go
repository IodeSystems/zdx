package e2e

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestApi_IssueCloseRecordsCompletedSha covers IS-1062 acceptance: the close
// endpoint accepts and persists completed_in_sha + closed_dirty, and `issue
// show` surfaces them.
//
// CLI-level clean-tree gate is unit-tested in
// internal/cli/project/issue_test.go (TestIsWorkingTreeDirty); the gate runs
// before this endpoint is hit, so this test focuses on the server contract.
func TestApi_IssueCloseRecordsCompletedSha(t *testing.T) {
	slug := "e2e-close-audit"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Close Audit"}, nil))

	addIssue := func(title string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      title,
				"context":    "audit-recording test",
				"issue_type": "impl",
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}

	// Satisfy unrelated close gates (resolution + substantive work-log)
	// so the audit field write is what we're isolating.
	prep := func(id int32) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{"slug": slug, "id": id, "shas": []string{"deadbeef"}, "source": "manual"}, nil))
		mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
			map[string]any{"issue_id": id, "by_role": "test", "note": "completed the work"}, nil))
	}

	type issueShape struct {
		Issue struct {
			ID             int32  `json:"id"`
			Status         string `json:"status"`
			CompletedInSha string `json:"completed_in_sha"`
			ClosedDirty    bool   `json:"closed_dirty"`
		} `json:"issue"`
	}

	getIssue := func(id int32) issueShape {
		t.Helper()
		var out issueShape
		mustOK(t, apiDo(t, http.MethodGet,
			"/api/dx/todo/issue/show?slug="+slug+"&id="+fmt.Sprintf("IS-%d", id), nil, &out))
		return out
	}

	// Case 1: clean close — sha recorded, closed_dirty=false (omitempty on
	// JSON => the unmarshal decoder leaves it false).
	id1 := addIssue("clean close")
	prep(id1)
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{
			"slug":             slug,
			"id":               id1,
			"reason":           "done",
			"completed_in_sha": "abc123def456",
			"closed_dirty":     false,
		}, nil))
	got := getIssue(id1)
	if got.Issue.Status != "closed" {
		t.Errorf("clean close: status=%q, want closed", got.Issue.Status)
	}
	if got.Issue.CompletedInSha != "abc123def456" {
		t.Errorf("clean close: completed_in_sha=%q, want abc123def456", got.Issue.CompletedInSha)
	}
	if got.Issue.ClosedDirty {
		t.Errorf("clean close: closed_dirty=true, want false")
	}

	// Case 2: dirty force-close — sha recorded, closed_dirty=true persists.
	id2 := addIssue("dirty force close")
	prep(id2)
	tru := true
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{
			"slug":             slug,
			"id":               id2,
			"reason":           "done",
			"force":            true,
			"completed_in_sha": "feedface00",
			"closed_dirty":     tru,
		}, nil))
	got = getIssue(id2)
	if got.Issue.CompletedInSha != "feedface00" {
		t.Errorf("dirty close: completed_in_sha=%q, want feedface00", got.Issue.CompletedInSha)
	}
	if !got.Issue.ClosedDirty {
		t.Errorf("dirty close: closed_dirty=false, want true")
	}

	// Case 3: --closed-dirty filter on list-issues returns only id2.
	type listShape struct {
		Issues []struct {
			ID          int32 `json:"id"`
			ClosedDirty bool  `json:"closed_dirty"`
		} `json:"issues"`
	}
	var ls listShape
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/todo/issue/list?slug="+slug+"&closed_dirty=true", nil, &ls))
	if len(ls.Issues) != 1 {
		t.Fatalf("list closed_dirty: got %d issues, want 1", len(ls.Issues))
	}
	if ls.Issues[0].ID != id2 {
		t.Errorf("list closed_dirty: got id=%d, want id=%d", ls.Issues[0].ID, id2)
	}
	if !ls.Issues[0].ClosedDirty {
		t.Errorf("list closed_dirty: row carries closed_dirty=false")
	}
}

// TestApi_IssueCloseCommitMessageAudit covers IS-1083: when a project has a
// git config and the operator asserts completed_in_sha on a non-force "done"
// close, the close handler resolves the sha in the project repo and rejects
// the close (409) unless the commit's title/body/trailers reference the
// issue ID being closed. Force-bypass and categorical reasons (duplicate /
// wontfix / superseded / link) skip the audit by design.
func TestApi_IssueCloseCommitMessageAudit(t *testing.T) {
	slug := "e2e-close-audit-msg"
	// Repos/<slug> is owned by the server (RepoDir) and lives at the
	// project root, not under t.TempDir(). Clear any clone from a prior
	// run so EnsureRepo creates a fresh checkout pointing at this run's
	// source repo.
	for _, base := range []string{"repos", "../e2e/repos", filepath.Join(os.Getenv("ZDX_HOME"), "data/repos")} {
		if base == "" {
			continue
		}
		_ = os.RemoveAll(filepath.Join(base, slug))
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Close Audit Msg"}, nil))

	// Build a local source repo. The audit code path in the handler runs
	// EnsureRepo which clones single-branch from origin and tracks via
	// fetch+reset, so a plain file:// remote with commits on `main` is
	// sufficient.
	src := t.TempDir()
	gitInit := func(args ...string) string {
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
	gitInit("init", "-q", "-b", "main")
	gitInit("config", "user.email", "t@e2e")
	gitInit("config", "user.name", "t")
	// Initial commit so the branch exists for clone.
	mustWrite := func(name, body string) {
		t.Helper()
		path := filepath.Join(src, name)
		if err := exec.Command("sh", "-c", "echo '"+body+"' > "+path).Run(); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	mustWrite("README", "init")
	gitInit("add", ".")
	gitInit("commit", "-q", "-m", "init")

	// Configure project git so the handler clones from src into RepoDir.
	mustOK(t, apiDo(t, http.MethodPut, "/api/admin/project-git-config",
		map[string]any{"slug": slug, "git_url": "file://" + src, "git_branch": "main"}, nil))

	addIssue := func(title string) int32 {
		t.Helper()
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
			map[string]any{
				"slug":       slug,
				"title":      title,
				"context":    "audit-msg test",
				"issue_type": "impl",
				"auto_ready": true,
			}, &resp))
		return resp.ID
	}
	prep := func(id int32) {
		t.Helper()
		mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/resolve",
			map[string]any{"slug": slug, "id": id, "shas": []string{"deadbeef"}, "source": "manual"}, nil))
		mustOK(t, apiDo(t, http.MethodPost, "/api/issue-work",
			map[string]any{"issue_id": id, "by_role": "test", "note": "completed the work"}, nil))
	}

	// Helper: add a new commit with a custom message and return its sha.
	mkCommit := func(name, msg string) string {
		t.Helper()
		mustWrite(name, "x")
		gitInit("add", ".")
		gitInit("commit", "-q", "-m", msg)
		return gitInit("rev-parse", "HEAD")
	}

	doClose := func(id int32, body map[string]any) *http.Response {
		t.Helper()
		body["slug"] = slug
		body["id"] = id
		return apiDo(t, http.MethodPost, "/api/dx/todo/issue/close", body, nil)
	}

	// Case 1: matched-title — subject prefixed with "IS-N:" passes.
	id1 := addIssue("commit references issue in title")
	prep(id1)
	sha1 := mkCommit("a1", fmt.Sprintf("IS-%d: implement the thing", id1))
	mustOK(t, doClose(id1, map[string]any{"reason": "done", "completed_in_sha": sha1}))

	// Case 2: matched-body — IS-N anywhere in the message body.
	id2 := addIssue("commit references issue in body")
	prep(id2)
	sha2 := mkCommit("a2", fmt.Sprintf("implement the thing\n\nFixes IS-%d as part of the cleanup.", id2))
	mustOK(t, doClose(id2, map[string]any{"reason": "done", "completed_in_sha": sha2}))

	// Case 3: matched-trailer — Issue: IS-N trailer.
	id3 := addIssue("commit references issue in trailer")
	prep(id3)
	sha3 := mkCommit("a3", fmt.Sprintf("subject\n\nbody line\n\nIssue: IS-%d", id3))
	mustOK(t, doClose(id3, map[string]any{"reason": "done", "completed_in_sha": sha3}))

	// Case 4: unmatched — message references a different issue → 409.
	id4 := addIssue("commit message names other issue")
	prep(id4)
	wrong := id4 + 1000 // a different number; pseudo-issue id
	sha4 := mkCommit("a4", fmt.Sprintf("IS-%d: drift fix unrelated to id4", wrong))
	resp := doClose(id4, map[string]any{"reason": "done", "completed_in_sha": sha4})
	if resp.StatusCode != 409 {
		t.Errorf("unmatched: want 409 got %d", resp.StatusCode)
	}
	// Confirm the issue is still open (close rejected, state unchanged).
	type stateShape struct {
		Issue struct {
			Status string `json:"status"`
		} `json:"issue"`
	}
	var s stateShape
	mustOK(t, apiDo(t, http.MethodGet,
		"/api/dx/todo/issue/show?slug="+slug+"&id="+fmt.Sprintf("IS-%d", id4), nil, &s))
	if s.Issue.Status != "open" {
		t.Errorf("unmatched: issue should still be open, got %q", s.Issue.Status)
	}

	// Case 5: unresolvable-sha — sha not in the repo → 409.
	id5 := addIssue("bogus sha")
	prep(id5)
	resp = doClose(id5, map[string]any{"reason": "done", "completed_in_sha": "deadbeef00deadbeef00deadbeef00deadbeef00"})
	if resp.StatusCode != 409 {
		t.Errorf("unresolvable: want 409 got %d", resp.StatusCode)
	}

	// Case 6: force bypass — audit skipped even though the commit message is
	// unmatched. The force marker records the override.
	id6 := addIssue("force bypass with unmatched commit")
	prep(id6)
	sha6 := mkCommit("a6", "no issue reference here")
	mustOK(t, doClose(id6, map[string]any{
		"reason":           "rollback",
		"force":            true,
		"completed_in_sha": sha6,
	}))

	// Case 7: reason=wontfix bypass — categorical reason, audit skipped even
	// though the sha is bogus (in fact wontfix typically has no sha; we still
	// verify the audit doesn't fire when sha is supplied with --force +
	// categorical reason).
	id7 := addIssue("wontfix close")
	prep(id7)
	mustOK(t, doClose(id7, map[string]any{
		"reason":           "wontfix",
		"force":            true,
		"completed_in_sha": "deadbeef",
	}))
}
