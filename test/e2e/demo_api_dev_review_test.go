package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestDemoAPI_DevReviewSetsReviewedAt is the demo for spec 78 on feature
// dx-todo-tasks: given a done task, when POST /api/dx/todo/dev/review is
// called with verdict=approve or verdict=reject, then reviewed_at is set on
// the task and a non-zero review_id is returned.
//
// The WS task.reviewed event publication is covered by the companion
// integration test TestDevReviewEmitsTaskReviewedEvent.
func TestDemoAPI_DevReviewSetsReviewedAt(t *testing.T) {
	rec := newApiRecorder(t, "dev-review-sets-reviewed-at")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_dev_review_test.go", Note: "dev-review demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_tasks.go", Note: "POST /api/dx/todo/dev/review — sets reviewed_at and publishes task.reviewed"})
	rec.AddCoderef(coderef{FilePath: "queries/tasks.sql", Note: "MarkTaskReviewed sets reviewed_at = NOW()"})
	t.Cleanup(rec.Save)

	const slug = "demo-dev-review"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Dev Review",
	}, nil))

	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       slug,
		"title":      "Review verdict coverage",
		"context":    "spec 78",
		"auto_ready": true,
	}, &issue))

	issueRef := fmt.Sprintf("IS-%d", issue.ID)

	addTask := func(text string) int32 {
		var resp struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
			"slug":       slug,
			"text":       text,
			"issue":      issueRef,
			"auto_ready": true,
			"force":      true,
		}, &resp))
		return resp.ID
	}

	approveID := addTask("approve target task")
	rejectID := addTask("reject target task")

	// Mark both tasks done before review.
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/dev/done", map[string]any{"id": approveID}, nil))
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/dev/done", map[string]any{"id": rejectID}, nil))

	// POST review with verdict=approve.
	var approveResp struct {
		OK       bool  `json:"ok"`
		ReviewID int32 `json:"review_id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/dev/review", map[string]any{
		"slug":    slug,
		"id":      approveID,
		"verdict": "approve",
	}, &approveResp))
	if !approveResp.OK || approveResp.ReviewID == 0 {
		t.Fatalf("approve: want ok=true review_id>0, got ok=%v review_id=%d", approveResp.OK, approveResp.ReviewID)
	}

	// POST review with verdict=reject.
	var rejectResp struct {
		OK       bool  `json:"ok"`
		ReviewID int32 `json:"review_id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/dev/review", map[string]any{
		"slug":    slug,
		"id":      rejectID,
		"verdict": "reject",
	}, &rejectResp))
	if !rejectResp.OK || rejectResp.ReviewID == 0 {
		t.Fatalf("reject: want ok=true review_id>0, got ok=%v review_id=%d", rejectResp.OK, rejectResp.ReviewID)
	}

	// Verify reviewed_at is set for both tasks via review-data.
	checkReviewedAt := func(taskID int32, verdict string) {
		t.Helper()
		var data struct {
			Task struct {
				ReviewedAt string `json:"reviewed_at"`
			} `json:"task"`
		}
		mustOK(t, rec.Do(http.MethodGet,
			fmt.Sprintf("/api/dx/todo/dev/review-data?slug=%s&id=%d", slug, taskID),
			nil, &data))
		if data.Task.ReviewedAt == "" {
			t.Errorf("verdict=%s task %d: reviewed_at not set", verdict, taskID)
		}
	}
	checkReviewedAt(approveID, "approve")
	checkReviewedAt(rejectID, "reject")
}
