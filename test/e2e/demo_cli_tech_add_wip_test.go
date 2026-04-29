package e2e

import (
	"strings"
	"testing"
)

// TestDemoCLI_TechAddWipDefault is the demo for spec 74: when `dx todo tech add`
// is run without --auto-ready and similar open tasks exist, the new task is
// created as wip, similar tasks are surfaced, and explicit promotion via
// `dx task ready` is required to move it to ready.
//
// Mirrors the integration coverage in TestTaskTechAddWithoutAutoReady but
// drives the full flow through the CLI so the recorded transcript shows the
// human-facing experience.
func TestDemoCLI_TechAddWipDefault(t *testing.T) {
	const name = "tech-add-wip-default"

	// Variable embedding mock yields cosine 0.5 between alpha- and beta-tagged
	// texts: high enough to surface as "similar" but below the 0.90 duplicate
	// block threshold, so the task is created (in wip) rather than refused.
	url := startVariableEmbedMock(t)
	registerEmbedMock(t, "spec74-demo-llm", url)

	rec := newRecorder(t, name, "bin/dx")
	t.Cleanup(rec.Save)
	slug := "demo-" + name

	rec.Run("issue", "add",
		"--title=Auth endpoint reference docs",
		"--auto-ready",
	)
	issueID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if issueID == "" {
		t.Fatal("could not extract issue ID")
	}

	// Seed: an already-ready task on the issue so the next add has a similar
	// neighbour to surface.
	const seedText = "alpha endpoint reference for /api/auth"
	rec.Run("todo", "tech", "add",
		"--issue="+issueID,
		"--text="+seedText,
		"--auto-ready",
	)
	seedID := extractFirstID(rec.steps[len(rec.steps)-1].Stdout)
	if seedID == "" {
		t.Fatal("could not extract seed task ID")
	}
	waitForTaskIndexed(t, slug, seedText)

	// Spec 74: tech add WITHOUT --auto-ready and with a similar task present
	// must (a) print the similar-tasks panel, (b) instruct the user to run
	// `dx task ready`, and (c) leave the new task in wip.
	rec.Run("todo", "tech", "add",
		"--issue="+issueID,
		"--text=beta description of /api/auth handler shape",
	)
	wipOut := rec.steps[len(rec.steps)-1].Stdout
	wipTaskID := extractFirstID(wipOut)
	if wipTaskID == "" {
		t.Fatalf("could not extract wip task ID from output:\n%s", wipOut)
	}
	if strings.Contains(wipOut, "auto-promoted to ready") {
		t.Errorf("spec 74: similar-tasks path must not auto-promote, got:\n%s", wipOut)
	}
	if !strings.Contains(wipOut, "Similar tasks:") {
		t.Errorf("spec 74: expected 'Similar tasks:' panel, got:\n%s", wipOut)
	}
	if !strings.Contains(wipOut, "Task created as draft (wip)") {
		t.Errorf("spec 74: expected wip-draft notice, got:\n%s", wipOut)
	}
	if !strings.Contains(wipOut, "dx task ready "+wipTaskID) {
		t.Errorf("spec 74: expected promotion hint with `dx task ready %s`, got:\n%s", wipTaskID, wipOut)
	}

	// Confirm persisted state via `dx todo show`: status must still be wip.
	rec.Run("todo", "show", wipTaskID)
	showWip := rec.steps[len(rec.steps)-1].Stdout
	if !strings.Contains(showWip, "Status:   wip") {
		t.Errorf("spec 74: persisted status should be wip before promotion, got:\n%s", showWip)
	}

	// Explicit promotion — the only way out of wip on this path.
	rec.Run("task", "ready", wipTaskID)
	if rec.steps[len(rec.steps)-1].ExitCode != 0 {
		t.Fatalf("`dx task ready %s` failed: %s", wipTaskID, rec.steps[len(rec.steps)-1].Stderr)
	}

	rec.Run("todo", "show", wipTaskID)
	showReady := rec.steps[len(rec.steps)-1].Stdout
	if !strings.Contains(showReady, "Status:   ready") {
		t.Errorf("spec 74: status must flip to ready after `dx task ready`, got:\n%s", showReady)
	}

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}
