package e2e

import (
	"fmt"
	"math"
	"net/http"
	"testing"
	"time"
)

// TestDemoAPI_StandupCadenceScalesWithVelocity is the demo for spec 84 on
// feature dx-todo-queue: given completed tasks in a project, when solo evaluates
// standup cadence, check-in frequency scales with velocity — min 7 days, max 30
// days, inversely proportional to closed task count.
//
// Strategy: POST a journal checkin dated 28 days ago. With zero closed tasks the
// cadence is 30 days and the entry is fresh (28 < 30). After closing 11 tasks the
// cadence drops to ≈27.27 days (30 / max(1, 11/10) = 30/1.1) and the same entry
// becomes overdue (28 > 27.27). GET /api/dx/solo/health is the data source the
// CLI uses to drive this decision.
func TestDemoAPI_StandupCadenceScalesWithVelocity(t *testing.T) {
	rec := newApiRecorder(t, "standup-cadence-velocity")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_standup_cadence_test.go", Note: "standup-cadence-velocity demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/work/todo.go", Note: "journalOverdue: cadence = 30/max(1,closedTasks/10), floor 7 days"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_dx.go", Note: "GET /api/dx/solo/health returns closed_task_count and journal dates"})
	t.Cleanup(rec.Save)

	const slug = "demo-cadence-velocity"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Standup Cadence Velocity",
	}, nil))

	date28daysAgo := time.Now().AddDate(0, 0, -28).Format("2006-01-02")

	// Seed an owner journal entry dated 28 days ago.
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/journal/checkin", map[string]any{
		"slug":       slug,
		"role":       "owner",
		"date":       date28daysAgo,
		"tldr":       "initial checkin for cadence demo",
		"assessment": "project healthy",
		"concerns":   "none",
		"next":       "continue building",
	}, nil))

	// Phase 1: 0 closed tasks → cadence = 30 days. A 28-day-old entry is fresh.
	var health1 struct {
		ClosedTaskCount  int64  `json:"closed_task_count"`
		OwnerJournalDate string `json:"owner_journal_date"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/solo/health?slug="+slug, nil, &health1))
	if health1.ClosedTaskCount != 0 {
		t.Fatalf("phase 1: want closed_task_count=0, got %d", health1.ClosedTaskCount)
	}
	if health1.OwnerJournalDate != date28daysAgo {
		t.Fatalf("phase 1: want owner_journal_date=%q, got %q", date28daysAgo, health1.OwnerJournalDate)
	}
	if cadenceOverdue(health1.OwnerJournalDate, health1.ClosedTaskCount) {
		t.Error("phase 1: 0 closed tasks → cadence=30d, 28-day-old entry must be fresh (not overdue)")
	}

	// Close 11 tasks: cadence = 30/max(1, 11/10) = 30/1.1 ≈ 27.27 days.
	var issue struct {
		ID int32 `json:"id"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/issue/add", map[string]any{
		"slug":       slug,
		"title":      "cadence demo tasks",
		"context":    "bulk closed tasks to drive velocity above cadence floor",
		"auto_ready": true,
	}, &issue))
	issueRef := fmt.Sprintf("IS-%d", issue.ID)

	for i := range 11 {
		var task struct {
			ID int32 `json:"id"`
		}
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/tech/add", map[string]any{
			"slug":       slug,
			"text":       fmt.Sprintf("cadence task %d", i+1),
			"issue":      issueRef,
			"auto_ready": true,
			"force":      true,
		}, &task))
		mustOK(t, rec.Do(http.MethodPost, "/api/dx/todo/dev/done", map[string]any{
			"id":        task.ID,
			"test_plan": "done for cadence demo",
		}, nil))
	}

	// Phase 2: 11 closed tasks → cadence ≈ 27.27 days. Same 28-day-old entry is overdue.
	var health2 struct {
		ClosedTaskCount  int64  `json:"closed_task_count"`
		OwnerJournalDate string `json:"owner_journal_date"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/solo/health?slug="+slug, nil, &health2))
	if health2.ClosedTaskCount != 11 {
		t.Fatalf("phase 2: want closed_task_count=11, got %d", health2.ClosedTaskCount)
	}
	if !cadenceOverdue(health2.OwnerJournalDate, health2.ClosedTaskCount) {
		t.Error("phase 2: 11 closed tasks → cadence≈27.27d, 28-day-old entry must be overdue")
	}
}

// cadenceOverdue mirrors journalOverdue in internal/cli/work/todo.go.
// cadence = 30 / max(1, closedTasks/10), floored at 7 days.
func cadenceOverdue(dateStr string, closedTasks int64) bool {
	if dateStr == "" {
		return true
	}
	t, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return true
	}
	cadence := 30.0 / math.Max(1.0, float64(closedTasks)/10.0)
	if cadence < 7 {
		cadence = 7
	}
	return time.Since(t).Hours()/24 > cadence
}
