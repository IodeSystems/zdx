package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_AgentEvaluateDiff is the demo for spec 66 on feature
// dx-todo-persistence: given an existing persisted queue, when
// /api/dx/agent/evaluate runs, then a diff is returned with four buckets
// (added/removed/changed/unchanged) comparing the persisted todos to a freshly
// generated queue.
//
// Strategy mirrors the integration test (TestAgentEvaluateDiff): seed three
// untriaged issues so the generator proposes triage-IS-N candidates, persist a
// deliberately divergent snapshot via /api/dx/agent/apply (one exact, one with
// a wrong priority, one omitted, one fabricated), then evaluate and inspect
// each bucket.
func TestDemoAPI_AgentEvaluateDiff(t *testing.T) {
	rec := newApiRecorder(t, "agent-evaluate-diff")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_solo_evaluate_diff_test.go", Note: "agent-evaluate-diff demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_solo.go", LineStart: 809, LineEnd: 870, Note: "POST /api/dx/agent/evaluate computes added/removed/changed/unchanged"})
	t.Cleanup(rec.Save)

	const slug = "demo-evaluate-diff"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Agent Evaluate Diff",
	}, nil))

	addIssue := func(title, ctx string) int32 {
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
			"slug": slug, "title": title, "context": ctx, "auto_ready": true,
		}, &resp))
		return resp.ID
	}
	idX := addIssue("Issue X unchanged", "applied with the proposed values")
	idY := addIssue("Issue Y stale", "applied with a deliberately wrong priority")
	idZ := addIssue("Issue Z new", "omitted from apply so evaluate flags it as Added")

	keyX := fmt.Sprintf("triage-IS-%d", idX)
	keyY := fmt.Sprintf("triage-IS-%d", idY)
	keyZ := fmt.Sprintf("triage-IS-%d", idZ)
	const keyFake = "evaluate-diff-stale-sentinel"

	// Each issue/add fires refreshQueueAsync; let the goroutines finish before
	// we overwrite the persisted queue via /apply.
	time.Sleep(50 * time.Millisecond)

	// Capture the proposed queue once so we can craft a divergent snapshot.
	var initial struct {
		Added     []AgentQueueItem `json:"added"`
		Removed   []TodoItem       `json:"removed"`
		Changed   []EvaluateChange `json:"changed"`
		Unchanged []AgentQueueItem `json:"unchanged"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/evaluate", map[string]any{
		"slug": slug, "issue": "",
	}, &initial))

	proposed := append([]AgentQueueItem{}, initial.Added...)
	proposed = append(proposed, initial.Unchanged...)
	for _, c := range initial.Changed {
		proposed = append(proposed, c.After)
	}

	// Build the apply payload: X exact, Y bumped to priority 5, Z omitted, plus
	// a fabricated key the generator will never produce. Y's bump must be
	// LOWER (more urgent) than the natural triage priority of 20: UpsertTodo
	// uses LEAST(existing, EXCLUDED) so an operator-style bump survives
	// re-evaluate (commit 2625eb18); a higher number is silently dropped and
	// the diff has nothing to detect.
	var applyItems []AgentQueueItem
	for _, it := range proposed {
		switch it.Key {
		case keyX:
			applyItems = append(applyItems, it)
		case keyY:
			bumped := it
			bumped.Priority = 5
			applyItems = append(applyItems, bumped)
		case keyZ:
			// omit → Added
		default:
			applyItems = append(applyItems, it)
		}
	}
	applyItems = append(applyItems, AgentQueueItem{
		Key: keyFake, Kind: "triage", Text: "sentinel from a prior evaluation",
		TargetType: "project", TargetID: slug,
		Priority: 50, Persona: "owner", Status: "open",
	})

	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/apply", map[string]any{
		"slug": slug, "items": applyItems,
	}, nil))

	// Evaluate again — the response is the diff between the persisted snapshot
	// and the freshly generated proposal.
	var diff EvaluateDiffResult
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/evaluate", map[string]any{
		"slug": slug, "issue": "",
	}, &diff))

	containsKey := func(items []AgentQueueItem, key string) bool {
		for _, it := range items {
			if it.Key == key {
				return true
			}
		}
		return false
	}
	containsTodoKey := func(items []TodoItem, key string) bool {
		for _, it := range items {
			if it.Key == key {
				return true
			}
		}
		return false
	}
	containsChangedKey := func(items []EvaluateChange, key string) bool {
		for _, it := range items {
			if it.After.Key == key {
				return true
			}
		}
		return false
	}

	if !containsKey(diff.Added, keyZ) {
		t.Errorf("Added: want %q, got %+v", keyZ, diff.Added)
	}
	if !containsTodoKey(diff.Removed, keyFake) {
		t.Errorf("Removed: want %q, got %+v", keyFake, diff.Removed)
	}
	if !containsChangedKey(diff.Changed, keyY) {
		t.Errorf("Changed: want %q, got %+v", keyY, diff.Changed)
	}
	if !containsKey(diff.Unchanged, keyX) {
		t.Errorf("Unchanged: want %q, got %+v", keyX, diff.Unchanged)
	}
}
