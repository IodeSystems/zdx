package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// IS-495: dx todo tech add must refuse to file a second open task whose
// title shares a templated stem ("Test spec N:") with an existing
// wip/ready/active task. Without this, the [tech:test-ref] nudge re-fires
// each session and spawns duplicate per-spec test tasks.

type stemDedupeResp struct {
	ID               int32 `json:"id"`
	DuplicateBlocked bool  `json:"duplicate_blocked"`
	Similar          []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"similar"`
}

func TestTechAddStemDedupeBlocksDuplicate(t *testing.T) {
	slug := "e2e-tech-add-stem-dedupe"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Stem Dedupe"}, nil)

	var first stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 23: Given demo recordings, verify A",
			"text":       "first approach",
			"auto_ready": true,
		}, &first))
	if first.ID == 0 {
		t.Fatalf("first add: expected task id, got 0")
	}

	var second stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":  slug,
			"title": "Test spec 23: Given demo recordings, verify B",
			"text":  "second approach with different body so embedding similarity stays low",
		}, &second))
	if !second.DuplicateBlocked {
		t.Fatalf("second add: expected duplicate_blocked=true, got false (id=%d)", second.ID)
	}
	if second.ID != 0 {
		t.Errorf("second add: expected no task created, got id=%d", second.ID)
	}
	if len(second.Similar) == 0 {
		t.Fatalf("second add: expected at least one similar task in response")
	}
	if !strings.HasPrefix(strings.ToLower(second.Similar[0].Title), "test spec 23:") {
		t.Errorf("similar[0] title should start with 'Test spec 23:'; got %q", second.Similar[0].Title)
	}
}

func TestTechAddStemDedupeAllowsDifferentSpec(t *testing.T) {
	slug := "e2e-tech-add-stem-dedupe-different"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Stem Dedupe Different"}, nil)

	var first stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 23: Given demo recordings",
			"text":       "first",
			"auto_ready": true,
		}, &first))

	var second stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 24: Given demo recordings",
			"text":       "second",
			"auto_ready": true,
		}, &second))
	if second.DuplicateBlocked {
		t.Fatalf("different spec number must not block; got duplicate_blocked=true")
	}
	if second.ID == 0 {
		t.Fatalf("different spec number: expected task id, got 0")
	}
}

func TestTechAddStemDedupeForceBypasses(t *testing.T) {
	slug := "e2e-tech-add-stem-dedupe-force"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Stem Dedupe Force"}, nil)

	var first stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 7: original",
			"text":       "first",
			"auto_ready": true,
		}, &first))

	var second stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 7: forced duplicate",
			"text":       "second",
			"force":      true,
			"auto_ready": true,
		}, &second))
	if second.DuplicateBlocked {
		t.Fatalf("--force must bypass stem dedupe; got duplicate_blocked=true")
	}
	if second.ID == 0 {
		t.Fatalf("--force add: expected task id, got 0")
	}
}

func TestTechAddStemDedupeIgnoresClosedTasks(t *testing.T) {
	slug := "e2e-tech-add-stem-dedupe-closed"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Stem Dedupe Closed"}, nil)

	var first stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 9: original",
			"text":       "first",
			"auto_ready": true,
		}, &first))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/dev/done",
		map[string]any{"id": first.ID, "test_plan": "covered"}, nil))

	var second stemDedupeResp
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"title":      "Test spec 9: a fresh take",
			"text":       "second",
			"auto_ready": true,
		}, &second))
	if second.DuplicateBlocked {
		t.Fatalf("closed task must not block stem dedupe; got duplicate_blocked=true")
	}
	if second.ID == 0 {
		t.Fatalf("after-close add: expected task id, got 0")
	}
}
