package e2e

import (
	"fmt"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_AgentApplyUpsertsAndResolvesStale is the demo for spec 67 on feature
// dx-todo-persistence: given a set of proposed queue items, when agent --apply is
// run, items are upserted by key and stale items not in the set are resolved
// atomically.
//
// Strategy: seed two untriaged issues so the evaluator proposes triage candidates
// for both (items_A). Apply items_A — all become open. Close issue 2 so the
// evaluator stops generating its candidates. Build items_B = items_A minus issue
// 2's items. Apply items_B — issue 2's items must be resolved atomically while
// items_B remain open.
func TestDemoAPI_AgentApplyUpsertsAndResolvesStale(t *testing.T) {
	rec := newApiRecorder(t, "agent-apply-upsert-stale")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_agent_apply_upsert_test.go", Note: "agent-apply-upsert-stale demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_agent_queue.go", LineStart: 965, LineEnd: 1016, Note: "POST /api/dx/agent/queue/apply upserts proposed items and resolves stale"})
	t.Cleanup(rec.Save)

	const slug = "demo-apply-upsert"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Agent Apply Upsert Stale",
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

	issueID1 := addIssue("Apply upsert issue one", "first issue for apply-upsert demo")
	issueID2 := addIssue("Apply upsert issue two", "second issue — will be closed to generate stale items")
	issue2Ref := fmt.Sprintf("IS-%d", issueID2)

	// Let refreshQueueAsync goroutines settle before evaluating.
	time.Sleep(50 * time.Millisecond)

	// Step 1: Evaluate to collect items_A (full proposed set).
	var evalResp struct {
		Added     []AgentQueueItem `json:"added"`
		Unchanged []AgentQueueItem `json:"unchanged"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/queue/evaluate", map[string]any{
		"slug": slug, "issue": "",
	}, &evalResp))

	itemsA := append([]AgentQueueItem{}, evalResp.Added...)
	itemsA = append(itemsA, evalResp.Unchanged...)
	if len(itemsA) < 2 {
		t.Fatalf("expected >=2 items from evaluate, got %d", len(itemsA))
	}

	// Step 2: Apply items_A — all upserted as open.
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/queue/apply", map[string]any{
		"slug": slug, "items": itemsA,
	}, nil))

	// Step 3: Verify items_A are open.
	var openA []TodoItem
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/agent/queue?slug="+slug+"&status=open", nil, &openA))
	if len(openA) < 2 {
		t.Fatalf("after applying items_A, expected >=2 open todos, got %d", len(openA))
	}

	// Step 4: Close issue 2 — evaluator will no longer generate its candidates.
	mustOK(t, rec.Do(http.MethodPost, "/api/issue-work", map[string]any{
		"issue_id": issueID2, "by_role": "test", "note": "ready to close",
	}, nil))
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/close", map[string]any{
		"slug": slug, "id": issueID2, "reason": "done",
	}, nil))

	// Step 5: Build items_B = items_A minus issue 2's items.
	var itemsB []AgentQueueItem
	var droppedKeys []string
	for _, item := range itemsA {
		if item.TargetID == issue2Ref {
			droppedKeys = append(droppedKeys, item.Key)
		} else {
			itemsB = append(itemsB, item)
		}
	}
	if len(droppedKeys) == 0 {
		t.Fatal("no items found targeting issue 2 — cannot demo stale resolution")
	}

	// Step 6: Apply items_B — issue 2's items must be resolved atomically.
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/agent/queue/apply", map[string]any{
		"slug": slug, "items": itemsB,
	}, nil))

	// Step 7: Verify dropped items are resolved.
	var resolved []TodoItem
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/agent/queue?slug="+slug+"&status=resolved", nil, &resolved))

	resolvedKeySet := map[string]bool{}
	for _, item := range resolved {
		resolvedKeySet[item.Key] = true
	}
	for _, key := range droppedKeys {
		if !resolvedKeySet[key] {
			t.Errorf("item %q should be resolved after subset apply, but is missing from resolved list", key)
		}
	}

	// Step 8: Verify items_B remain open and dropped items are not.
	var openB []TodoItem
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/agent/queue?slug="+slug+"&status=open", nil, &openB))

	openKeySet := map[string]bool{}
	for _, item := range openB {
		openKeySet[item.Key] = true
	}
	for _, item := range itemsB {
		if !openKeySet[item.Key] {
			t.Errorf("item %q (items_B) should still be open after subset apply", item.Key)
		}
	}
	for _, key := range droppedKeys {
		if openKeySet[key] {
			t.Errorf("item %q (items_A \\ items_B) should NOT be open after subset apply", key)
		}
	}

	_ = issueID1
}
