package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// DemoRecorder runs dx CLI commands against the test server and records each
// interaction to a structured JSON log in .zdx/demo/cli/<name>.json.
type DemoRecorder struct {
	t     *testing.T
	name  string
	dxBin string
	env   []string
	steps []demoStep
	refs  []coderef
}

type demoStep struct {
	Cmd        string   `json:"cmd"`
	Args       []string `json:"args"`
	Stdout     string   `json:"stdout"`
	Stderr     string   `json:"stderr,omitempty"`
	ExitCode   int      `json:"exit_code"`
	DurationMs int64    `json:"duration_ms"`
}

type demoLog struct {
	Name       string     `json:"name"`
	RecordedAt string     `json:"recorded_at"`
	Steps      []demoStep `json:"steps"`
}

// newRecorder returns a recorder pointed at the test server.
// dxBin must be a path to the compiled dx binary (e.g. "bin/dx").
func newRecorder(t *testing.T, name, dxBin string) *DemoRecorder {
	t.Helper()
	root, err := findRoot()
	if err != nil {
		t.Fatalf("cannot find project root: %v", err)
	}
	abs := filepath.Join(root, dxBin)
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("dx binary not found at %q — run: go build -o %s ./cmd/dx/", abs, dxBin)
	}
	slug := "demo-" + name
	apiDo(t, "POST", "/api/project",
		map[string]string{"slug": slug, "name": "Demo " + name}, nil)
	return &DemoRecorder{
		t:     t,
		name:  name,
		dxBin: abs,
		env: append(os.Environ(),
			"DX_REMOTE_URL="+srv.URL,
			"DX_REMOTE_API_KEY="+srv.AdminToken,
			"DX_REMOTE_SLUG="+slug,
		),
	}
}

// Run executes a dx subcommand and records the interaction.
func (r *DemoRecorder) Run(args ...string) {
	r.t.Helper()
	cmd := exec.Command(r.dxBin, args...)
	cmd.Env = r.env

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	dur := time.Since(start).Milliseconds()

	code := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			code = ee.ExitCode()
		}
	}

	r.steps = append(r.steps, demoStep{
		Cmd:        "dx " + strings.Join(args, " "),
		Args:       args,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   code,
		DurationMs: dur,
	})

	if code != 0 {
		r.t.Logf("dx %s → exit %d\n%s", strings.Join(args, " "), code, stderr.String())
	}
}

// AddCoderef registers an additional file to attach to the test row post-run.
// The recorder always includes the CLI test source file itself on Save.
func (r *DemoRecorder) AddCoderef(ref coderef) {
	r.refs = append(r.refs, ref)
}

// Save writes the structured log and emits a coderefs sidecar that dx test's
// syncTestResults will use to call AttachCodeRefToTest. Called automatically
// via t.Cleanup.
func (r *DemoRecorder) Save() {
	root, _ := findRoot()
	dir := filepath.Join(root, ".zdx", "demo", "cli")
	if err := os.MkdirAll(dir, 0755); err != nil {
		r.t.Logf("demo save: mkdir: %v", err)
		return
	}
	out := demoLog{
		Name:       r.name,
		RecordedAt: time.Now().UTC().Format(time.RFC3339),
		Steps:      r.steps,
	}
	b, _ := json.MarshalIndent(out, "", "  ")
	path := filepath.Join(dir, r.name+".json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		r.t.Logf("demo save: %v", err)
		return
	}
	r.t.Logf("CLI demo saved → %s", path)

	refs := append([]coderef{{
		FilePath: "test/e2e/demo_cli_test.go",
		Note:     "CLI demo source",
	}}, r.refs...)
	writeDemoCoderefs(r.t, r.t.Name(), refs)
}

// ── Demo tests ────────────────────────────────────────────────────────────────

func TestDemoCLI_ProjectAndIssueFlow(t *testing.T) {
	rec := newRecorder(t, "project-issue-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=First issue", "--context=Added via CLI demo")
	rec.Run("issue", "list")
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_TaskFlow(t *testing.T) {
	rec := newRecorder(t, "task-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Implement feature X")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Write the implementation")
	rec.Run("todo", "list", "--issue="+issueID)
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_BlockerQuestionFlow(t *testing.T) {
	rec := newRecorder(t, "blocker-question-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Decide on auth strategy", "--context=Need to pick OAuth provider")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	// Spec 1: question add with target-type, target-id, context → BQ-ID printed
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+issueID,
		"--context=Which OAuth provider should we use — Auth0, Cognito, or roll our own?",
	)
	bqID := extractFirstBQID(rec.steps[len(rec.steps)-1].Stdout)
	if bqID == "" {
		t.Errorf("expected BQ-ID in output, got: %s", rec.steps[len(rec.steps)-1].Stdout)
	}

	// Spec 9: question add with choices → choices persisted
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+issueID,
		"--context=Which database for the session store?",
		"--choices=Redis,DynamoDB,Postgres",
	)

	// Spec 3: question list → shows BQ-ID, target, status, context
	rec.Run("question", "list")

	// Spec 4: question list --status pending → only pending shown
	rec.Run("question", "list", "--status=pending")

	// Spec 89: solo in global mode excludes BQ-blocked issues from the queue.
	// Issue title should NOT appear in this output while questions are pending.
	rec.Run("todo", "solo")

	// Spec 90: solo with --issue=IS-N surfaces the pending question as clarify.
	rec.Run("todo", "solo", "--issue="+issueID)

	// Spec 2: question answer → status changes to answered
	if bqID != "" {
		numID := strings.TrimPrefix(bqID, "BQ-")
		rec.Run("question", "answer", numID, "--answer=Go with Auth0 for managed OAuth")
	}

	// Verify answered question no longer appears in pending list
	rec.Run("question", "list", "--status=pending")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_FeatureFlow(t *testing.T) {
	rec := newRecorder(t, "feature-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("feature", "add", "user-auth", "--desc=User authentication and session management")
	rec.Run("feature", "add", "data-export", "--desc=Export project data to CSV and JSON")
	rec.Run("feature", "list")
	rec.Run("feature", "show", "user-auth")
	rec.Run("feature", "set", "user-auth", "--category=Security", "--component=api")
	rec.Run("feature", "review", "user-auth")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

func TestDemoCLI_JournalFlow(t *testing.T) {
	rec := newRecorder(t, "journal-flow", "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("issue", "add", "--title=Implement journal feature", "--context=Work-log tracking via CLI")

	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Skip("could not extract issue ID from output")
	}

	// Spec 15: journal add with issue ID and note → work-log entry appended
	rec.Run("journal", "add", "--issue="+issueID, "--note=Started implementation of journal commands")
	rec.Run("journal", "add", "--issue="+issueID, "--note=Added add and list subcommands", "--role=dev")

	// Spec 16: journal list with issue ID → entries listed with timestamps and attribution
	rec.Run("journal", "list", "--issue="+issueID)

	// Also show unfiltered list
	rec.Run("journal", "list")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_UnifiedMixedSources is the demo for spec 99: dx todo solo integrates
// all signal sources (untriaged issues, unread comments, blocker questions, pending tasks,
// decomposition gaps, closable issues) into a single unified queue ordered by priority.
func TestDemoCLI_UnifiedMixedSources(t *testing.T) {
	rec := newRecorder(t, "unified-mixed-sources", "bin/dx")
	t.Cleanup(rec.Save)

	// Signal 1: untriaged issue → appears as "triage" in queue (priority 20)
	rec.Run("issue", "add", "--title=Bug: login fails on slow connections", "--auto-ready")
	untriagedID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)

	// Signal 2: unread comment → "read:comments" in queue (priority 5)
	rec.Run("issue", "add", "--title=Refactor auth middleware", "--auto-ready")
	commentIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if commentIssueID != "" {
		rec.Run("todo", "owner", "triage", commentIssueID, "--priority=2")
		rec.Run("comment", "add", "issue", commentIssueID, "--body=Should we also update the session store while we are at it?")
	}

	// Signal 3: blocker question → "clarify" in issue-scoped queue (priority 5)
	rec.Run("issue", "add", "--title=Choose caching strategy", "--auto-ready")
	bqIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if bqIssueID != "" {
		rec.Run("todo", "owner", "triage", bqIssueID, "--priority=2")
		rec.Run("question", "add", "--target-type=issue", "--target-id="+bqIssueID, "--context=Redis or Memcached?")
	}

	// Signal 4: decomposition gap — triaged, no tasks → "add" in queue (priority 38)
	rec.Run("issue", "add", "--title=Add rate limiting", "--auto-ready")
	decompID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if decompID != "" {
		rec.Run("todo", "owner", "triage", decompID, "--priority=2")
	}

	// Signal 5: pending task → "dev" in queue (priority 40)
	rec.Run("issue", "add", "--title=Write API documentation", "--auto-ready")
	devIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if devIssueID != "" {
		rec.Run("todo", "owner", "triage", devIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+devIssueID, "--text=Write endpoint reference for /api/auth")
	}

	// Signal 6: closable — all tasks done → "closable" in queue (priority 35)
	rec.Run("issue", "add", "--title=Update dependencies", "--auto-ready")
	closableIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if closableIssueID != "" {
		rec.Run("todo", "owner", "triage", closableIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+closableIssueID, "--text=Bump all packages to latest")
		taskID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
		if taskID != "" {
			rec.Run("todo", "dev", "done", taskID, "--test-plan=manually verified packages updated without errors")
		}
	}

	_ = untriagedID // appears as triage in global queue
	_ = decompID    // appears as add in global queue

	// Global queue: all signal sources merged into one prioritized view.
	rec.Run("todo", "solo")

	// Issue-scoped queue for BQ-blocked issue: clarify surfaces.
	if bqIssueID != "" {
		rec.Run("todo", "solo", "--issue="+bqIssueID)
	}

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_TodoDelegation is the demo for spec 100: the dx todo CLI commands
// (solo, list, show, take) delegate to server-side queue evaluation and persistence,
// so the CLI remains a thin view with no client-side queue logic divergence.
// The recorder configures DX_REMOTE_URL/SLUG — every command below goes over the
// API and reads server-persisted state.
func TestDemoCLI_TodoDelegation(t *testing.T) {
	rec := newRecorder(t, "todo-delegation", "bin/dx")
	t.Cleanup(rec.Save)

	// Seed: one triaged issue with a task so list/show/take have content.
	rec.Run("issue", "add", "--title=Delegation demo issue", "--auto-ready")
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID from output")
	}
	rec.Run("todo", "owner", "triage", issueID, "--priority=2")
	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Implement delegation")
	taskID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)

	// solo: server evaluates the queue and returns the next item.
	rec.Run("todo", "solo")

	// list: returns server-persisted tasks for the issue.
	rec.Run("todo", "list", "--issue="+issueID)

	// show: reads issue state from the server.
	rec.Run("todo", "show", issueID)
	if taskID != "" {
		rec.Run("todo", "show", taskID)
	}

	// take: atomic server-side reservation; response carries the claiming agent.
	rec.Run("todo", "take", "--agent-id=demo-agent", "--lease-minutes=5")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_SharedTodoQueue is the demo for spec 101: humans (dx todo solo)
// and agents (dx todo take) operate on the same persisted todo records with no
// separate agent-only queue. The demo shows a human evaluating the queue, an
// agent claiming from that same queue, and the human seeing the claimed item as
// unchanged (still the shared persisted row) on re-evaluation.
func TestDemoCLI_SharedTodoQueue(t *testing.T) {
	rec := newRecorder(t, "shared-todo-queue", "bin/dx")
	t.Cleanup(rec.Save)

	// Seed: an issue the human will see and the agent will claim.
	rec.Run("issue", "add", "--title=Implement shared queue feature", "--auto-ready")
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID from output")
	}
	rec.Run("todo", "owner", "triage", issueID, "--priority=2")
	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Write the shared queue logic")

	// Human path: dx todo solo evaluates and shows the queue.
	rec.Run("todo", "solo")

	// Agent path: dx todo take atomically claims the next item from the same queue.
	// The claimed item will be one of the same todos that solo just showed —
	// there is no separate agent-only queue.
	rec.Run("todo", "take", "--agent-id=human-agent-parity", "--lease-minutes=5")

	// Human re-evaluates: the claimed item is still in the persisted todo set
	// (claim preserves open status), so it appears as unchanged rather than new.
	rec.Run("todo", "solo")

	// Agent claims the next unclaimed item — demonstrating the queue is finite
	// and shared: once claimed by either caller, the item is not re-issued.
	rec.Run("todo", "take", "--agent-id=human-agent-parity", "--lease-minutes=5")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_TodoItemFields is the demo for spec 102: a todo item surfaced
// by the queue carries the owner tier, target entity reference, priority, and
// suggested CLI action — enough for the caller to act without further discovery.
// The demo seeds items across tiers (owner triage, tech add, dev task) so the
// queue output visibly spans all tiers, then claims one to show the structured
// take response including target_type/target_id and the claiming agent's lease.
func TestDemoCLI_TodoItemFields(t *testing.T) {
	rec := newRecorder(t, "todo-item-fields", "bin/dx")
	t.Cleanup(rec.Save)

	// Tier 1 (owner): an untriaged issue surfaces as [owner:triage].
	rec.Run("issue", "add", "--title=Needs owner triage", "--auto-ready")

	// Tier 2 (tech): a triaged-but-undecomposed issue surfaces as [tech:add].
	rec.Run("issue", "add", "--title=Needs tech decomposition", "--auto-ready")
	techIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if techIssueID != "" {
		rec.Run("todo", "owner", "triage", techIssueID, "--priority=2")
	}

	// Tier 3 (dev): a task under a triaged issue surfaces as [dev].
	rec.Run("issue", "add", "--title=Has a pending dev task", "--auto-ready")
	devIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if devIssueID != "" {
		rec.Run("todo", "owner", "triage", devIssueID, "--priority=3")
		rec.Run("todo", "tech", "add", "--issue="+devIssueID, "--text=Implement the behavior")
	}

	// Queue view: each line carries the tier ([owner:*]/[tech:*]/[dev]) and the
	// target reference (IS-N / TK-N) so the caller can dispatch directly.
	rec.Run("todo", "solo")

	// Claim view: take returns a structured item with kind (tier:action),
	// target_type:target_id, issue_ref, and the claiming agent's lease — the
	// caller can act on the response without another round-trip.
	rec.Run("todo", "take", "--agent-id=demo-fields", "--lease-minutes=5")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_SoloPriorityOrder is the demo for spec 62: when solo is run without
// --issue, items are returned in strict priority order:
// comments (5) < questions (10) < triage (20) < specs (25) < closable (35) < decomposition (38) < tasks (40).
// The demo seeds one representative item per tier so the solo output visibly
// steps through the full priority ladder in a single run.
func TestDemoCLI_SoloPriorityOrder(t *testing.T) {
	rec := newRecorder(t, "solo-priority-order", "bin/dx")
	t.Cleanup(rec.Save)

	// Priority 5 — unread comment: triaged issue with an unread comment.
	rec.Run("issue", "add", "--title=Auth middleware refactor", "--auto-ready")
	commentIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if commentIssueID != "" {
		rec.Run("todo", "owner", "triage", commentIssueID, "--priority=2")
		rec.Run("comment", "add", "issue", commentIssueID, "--body=Should we also update the session store?")
	}

	// Priority 10 — unanswered QA question (global, not issue-scoped).
	rec.Run("qa", "add", "--question=Which caching strategy should we adopt?")

	// Priority 20 — untriaged issue.
	rec.Run("issue", "add", "--title=Bug: dashboard crashes on load", "--auto-ready")

	// Priority 25 — feature with no specs.
	rec.Run("feature", "add", "notifications", "--desc=Push notification delivery")

	// Priority 35 — closable: all tasks done.
	rec.Run("issue", "add", "--title=Update CI pipeline", "--auto-ready")
	closableIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if closableIssueID != "" {
		rec.Run("todo", "owner", "triage", closableIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+closableIssueID, "--text=Bump runner image")
		taskID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
		if taskID != "" {
			rec.Run("todo", "dev", "done", taskID, "--test-plan=CI green after image bump")
		}
	}

	// Priority 38 — decomposition gap: triaged with no tasks.
	rec.Run("issue", "add", "--title=Add rate limiting", "--auto-ready")
	decompIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if decompIssueID != "" {
		rec.Run("todo", "owner", "triage", decompIssueID, "--priority=2")
	}

	// Priority 40 — pending task.
	rec.Run("issue", "add", "--title=Write API docs", "--auto-ready")
	devIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if devIssueID != "" {
		rec.Run("todo", "owner", "triage", devIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+devIssueID, "--text=Document /api/auth endpoints")
	}

	// Global queue: all tiers present; output must appear in ascending priority order.
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_IssueScopedQueue is the demo for spec 63: when solo is run with
// --issue=IS-N, only items scoped to that issue are returned. The demo seeds
// one issue per scoped kind (triage, add, dev, closable) and runs
// dx todo solo --issue=IS-N for each, showing scope isolation in action.
func TestDemoCLI_IssueScopedQueue(t *testing.T) {
	rec := newRecorder(t, "issue-scoped-queue", "bin/dx")
	t.Cleanup(rec.Save)

	// Kind: triage — untriaged issue surfaced only when scoped to it.
	rec.Run("issue", "add", "--title=Untriaged bug report", "--auto-ready")
	triagedIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)

	// Kind: add — triaged with no tasks yet (decomposition gap).
	rec.Run("issue", "add", "--title=Needs task decomposition", "--auto-ready")
	addIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if addIssueID != "" {
		rec.Run("todo", "owner", "triage", addIssueID, "--priority=2")
	}

	// Kind: dev — triaged with a pending task.
	rec.Run("issue", "add", "--title=Has pending implementation task", "--auto-ready")
	devIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if devIssueID != "" {
		rec.Run("todo", "owner", "triage", devIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+devIssueID, "--text=Implement the feature")
	}

	// Kind: closable — triaged with all tasks done.
	rec.Run("issue", "add", "--title=Ready to close", "--auto-ready")
	closableIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if closableIssueID != "" {
		rec.Run("todo", "owner", "triage", closableIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+closableIssueID, "--text=Write the fix")
		taskID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
		if taskID != "" {
			rec.Run("todo", "dev", "done", taskID, "--test-plan=manually verified fix works")
		}
	}

	// Issue-scoped queries: each returns only the item for that issue.
	if triagedIssueID != "" {
		rec.Run("todo", "solo", "--issue="+triagedIssueID)
	}
	if addIssueID != "" {
		rec.Run("todo", "solo", "--issue="+addIssueID)
	}
	if devIssueID != "" {
		rec.Run("todo", "solo", "--issue="+devIssueID)
	}
	if closableIssueID != "" {
		rec.Run("todo", "solo", "--issue="+closableIssueID)
	}

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_WipIssueBlocksSolo is the demo for spec 85: when solo is run
// with --issue=IS-N and that issue is still in wip status, the command exits
// non-zero with a message directing the user to promote it via dx issue ready.
func TestDemoCLI_WipIssueBlocksSolo(t *testing.T) {
	rec := newRecorder(t, "wip-issue-blocks-solo", "bin/dx")
	t.Cleanup(rec.Save)

	// Create the issue directly via the API so it stays in wip status.
	// The CLI auto-promotes issues when no similar matches are found, making
	// it impossible to reliably produce a wip issue through the CLI alone.
	var addResp struct {
		ID int32 `json:"id"`
	}
	apiDo(t, "POST", "/api/dx/todo/issue/add",
		map[string]any{"slug": "demo-wip-issue-blocks-solo", "title": "WIP issue not ready", "auto_ready": false},
		&addResp)
	if addResp.ID == 0 {
		t.Fatal("issue creation via API failed")
	}
	wipIssueID := fmt.Sprintf("IS-%d", addResp.ID)

	// solo --issue on a wip issue must fail with a descriptive error.
	rec.Run("todo", "solo", "--issue="+wipIssueID)
	soloStep := rec.steps[len(rec.steps)-1]
	if soloStep.ExitCode == 0 {
		t.Errorf("expected non-zero exit for wip issue, got 0; stdout: %s", soloStep.Stdout)
	}
	if !strings.Contains(soloStep.Stderr, "still in draft (wip)") {
		t.Errorf("expected 'still in draft (wip)' in stderr, got: %s", soloStep.Stderr)
	}
	if !strings.Contains(soloStep.Stderr, "dx issue ready") {
		t.Errorf("expected 'dx issue ready' in stderr, got: %s", soloStep.Stderr)
	}
}

// TestDemoCLI_QueueBQBlockedGlobal is the demo for spec 64: given an issue
// blocked by a pending blocker question, when solo is run in global mode,
// that issue's items are excluded from the queue. The demo seeds a triaged
// issue with a pending blocker question plus a distractor issue that should
// remain visible, then runs dx todo solo to show the BQ-blocked issue's
// dev/add/closable rows are suppressed globally — they only return once the
// question is answered.
func TestDemoCLI_QueueBQBlockedGlobal(t *testing.T) {
	rec := newRecorder(t, "queue-bq-blocked-global", "bin/dx")
	t.Cleanup(rec.Save)

	// Blocked issue: triaged with a pending task, then gated by a blocker question.
	rec.Run("issue", "add", "--title=Choose caching strategy", "--auto-ready")
	blockedIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if blockedIssueID == "" {
		t.Fatal("could not extract blocked issue ID from output")
	}
	rec.Run("todo", "owner", "triage", blockedIssueID, "--priority=2")
	rec.Run("todo", "tech", "add", "--issue="+blockedIssueID, "--text=Implement the chosen cache layer")
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+blockedIssueID,
		"--context=Redis or Memcached?",
	)
	bqID := extractFirstBQID(rec.steps[len(rec.steps)-1].Stdout)

	// Distractor issue: not blocked — proves the queue is not globally empty.
	rec.Run("issue", "add", "--title=Write API documentation", "--auto-ready")
	openIssueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if openIssueID != "" {
		rec.Run("todo", "owner", "triage", openIssueID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+openIssueID, "--text=Document /api/auth endpoints")
	}

	// Global solo: the BQ-blocked issue's add/dev/closable rows are suppressed;
	// the distractor issue's task still appears.
	rec.Run("todo", "solo")

	// Answer the blocker question; the previously-blocked issue rejoins the queue.
	if bqID != "" {
		rec.Run("question", "answer", strings.TrimPrefix(bqID, "BQ-"), "--answer=Redis")
	}
	rec.Run("todo", "solo")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_QueueBQClarifyScoped is the demo for spec 65: given an issue with
// pending blocker questions, when solo is run with --issue=IS-N scoped to that
// issue, the blocker questions surface as clarify items with priority 5. The demo
// seeds a triaged issue with a blocker question, shows clarify appearing in the
// issue-scoped queue but not in the global queue, then answers the question to
// confirm the clarify item disappears.
func TestDemoCLI_QueueBQClarifyScoped(t *testing.T) {
	rec := newRecorder(t, "queue-bq-clarify-scoped", "bin/dx")
	t.Cleanup(rec.Save)

	// Create a triaged issue that will receive a blocker question.
	rec.Run("issue", "add", "--title=Select database engine for analytics", "--auto-ready")
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID from output")
	}
	rec.Run("todo", "owner", "triage", issueID, "--priority=2")
	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Configure chosen database engine")

	// Add a blocker question: this gates the issue until answered.
	rec.Run("question", "add",
		"--target-type=issue",
		"--target-id="+issueID,
		"--context=ClickHouse or Redshift — which fits our latency budget?",
	)
	bqID := extractFirstBQID(rec.steps[len(rec.steps)-1].Stdout)

	// Global solo: BQ-blocked issue's tasks are suppressed — no clarify surfaced here.
	rec.Run("todo", "solo")

	// Issue-scoped solo: blocker question surfaces as clarify (priority 5).
	rec.Run("todo", "solo", "--issue="+issueID)

	// Answer the question; the clarify item should no longer appear.
	if bqID != "" {
		rec.Run("question", "answer", strings.TrimPrefix(bqID, "BQ-"), "--answer=ClickHouse — sub-100ms p99 on our workload")
	}
	rec.Run("todo", "solo", "--issue="+issueID)

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_QueueTrackerExcluded is the demo for spec 82: given a tracker-type
// issue, when solo evaluates the queue, the tracker is excluded because tracker
// issues are closed by their children, not worked directly. The demo seeds a
// tracker alongside a regular triaged issue with a pending task, then runs
// dx todo solo to show the tracker does not surface as triage/add/dev/closable
// while the regular issue's dev task still appears.
func TestDemoCLI_QueueTrackerExcluded(t *testing.T) {
	rec := newRecorder(t, "queue-tracker-excluded", "bin/dx")
	t.Cleanup(rec.Save)

	// Health prereqs so owner:goals / owner:constraints don't dominate the queue
	// and obscure the tracker-exclusion behavior under test.
	rec.Run("goal", "add", "Ship v1")
	rec.Run("constraint", "add", "No breaking changes")

	// Tracker issue with --auto-ready: without the tracker carve-out this would
	// otherwise surface as an [add] item (triaged but no tasks).
	rec.Run("issue", "add", "--title=Umbrella: Q2 platform hardening", "--type=tracker", "--auto-ready")

	// Distractor: a regular triaged issue with a pending task — proves the queue
	// isn't globally empty, and that non-tracker work is still surfaced.
	rec.Run("issue", "add", "--title=Harden rate limiter", "--auto-ready")
	regularID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if regularID != "" {
		rec.Run("todo", "owner", "triage", regularID, "--priority=2")
		rec.Run("todo", "tech", "add", "--issue="+regularID, "--text=Implement sliding-window limiter")
	}

	// Full queue evaluate: the tracker is absent from every regular kind
	// (triage/add/dev/closable); the distractor issue's [dev] task surfaces.
	rec.Run("todo", "solo", "--evaluate")

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// TestDemoCLI_ClosableIssueSurfaced is the demo for spec 83: given an issue where
// all linked tasks are done, when solo evaluates the queue, a closable item is
// surfaced at priority 35 indicating the issue can be closed.
func TestDemoCLI_ClosableIssueSurfaced(t *testing.T) {
	rec := newRecorder(t, "closable-issue-surfaced", "bin/dx")
	t.Cleanup(rec.Save)

	// Create and triage an issue.
	rec.Run("issue", "add", "--title=Upgrade TLS certificates", "--auto-ready")
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID from output")
	}
	rec.Run("todo", "owner", "triage", issueID, "--priority=2")

	// Add a task and complete it — all tasks done makes the issue closable.
	rec.Run("todo", "tech", "add", "--issue="+issueID, "--text=Renew certificates on all nodes")
	taskID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if taskID == "" {
		t.Fatal("could not extract task ID from output")
	}
	rec.Run("todo", "dev", "done", taskID, "--test-plan=certificates renewed and verified on all nodes")

	// Solo queue: with all tasks done, the issue surfaces as closable (priority 35).
	// The suggested action is dx issue close IS-N --reason=done.
	rec.Run("todo", "solo")

	// Issue-scoped: confirm closable surfaces when scoped to this specific issue.
	rec.Run("todo", "solo", "--issue="+issueID)

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}

// extractFirstBQID pulls the first BQ-N token from output.
func extractFirstBQID(output string) string {
	for _, word := range strings.Fields(output) {
		word = strings.Trim(word, ".,;:()")
		if strings.HasPrefix(word, "BQ-") {
			return word
		}
	}
	return ""
}

// TestDemoCLI_SoloBootstrapGuidance is the demo for spec 79: when dx todo solo
// is run on an empty project (no issues, no features), it emits a [bootstrap]
// line with the project slug and guidance directing the user toward onboarding.
func TestDemoCLI_SoloBootstrapGuidance(t *testing.T) {
	const name = "solo-bootstrap-cli"
	const slug = "demo-" + name
	rec := newRecorder(t, name, "bin/dx")
	t.Cleanup(rec.Save)

	rec.Run("todo", "solo")

	if len(rec.steps) == 0 {
		t.Fatal("no steps recorded")
	}
	combined := rec.steps[0].Stdout + rec.steps[0].Stderr

	if !strings.Contains(combined, "[bootstrap]") {
		t.Errorf("expected '[bootstrap]' in output, got:\n%s", combined)
	}
	if !strings.Contains(combined, slug) {
		t.Errorf("expected slug %q in output, got:\n%s", slug, combined)
	}
	if !strings.Contains(combined, "answer") && !strings.Contains(combined, "todo solo") {
		t.Errorf("expected onboarding guidance in output, got:\n%s", combined)
	}
}

// extractFirstID pulls the first IS-N or TK-N token from output.
func extractFirstID(output string) string {
	for _, word := range strings.Fields(output) {
		word = strings.Trim(word, ".,;:()")
		if strings.HasPrefix(word, "IS-") || strings.HasPrefix(word, "TK-") {
			return word
		}
	}
	return ""
}
