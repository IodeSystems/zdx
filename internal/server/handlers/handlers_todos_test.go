package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// stubTodoIncompleteStore implements TodoIncompleteStore for unit tests.
type stubTodoIncompleteStore struct {
	project       db.ZdxProject
	todo          db.GetTodoByKeyRow
	reports       []db.ZdxTodoIncompleteReport
	nextID        int64
	sideEffects   map[string]bool // key = "projectID:reason:fp:actionType"
	blocks        []db.AddIssueBlockParams
	questions     []db.ZdxQuestion
	issues        map[string]db.ZdxIssue
	issueCounter  int32
	testFixIssues map[string]string // key = "projectID:reason:fp" → issueID
	comments      []db.AddCommentParams
}

func (s *stubTodoIncompleteStore) GetProjectBySlug(_ context.Context, slug string) (db.ZdxProject, error) {
	if s.project.Slug != slug {
		return db.ZdxProject{}, fmt.Errorf("not found")
	}
	return s.project, nil
}

func (s *stubTodoIncompleteStore) GetTodoByKey(_ context.Context, arg db.GetTodoByKeyParams) (db.GetTodoByKeyRow, error) {
	if s.todo.ProjectID != arg.ProjectID || s.todo.Key != arg.Key {
		return db.GetTodoByKeyRow{}, fmt.Errorf("not found")
	}
	return s.todo, nil
}

func (s *stubTodoIncompleteStore) AddTodoIncompleteReport(_ context.Context, arg db.AddTodoIncompleteReportParams) (db.ZdxTodoIncompleteReport, error) {
	s.nextID++
	row := db.ZdxTodoIncompleteReport{
		ID:                  s.nextID,
		ProjectID:           arg.ProjectID,
		TodoID:              arg.TodoID,
		Reason:              arg.Reason,
		Explanation:         arg.Explanation,
		SuggestedNext:       arg.SuggestedNext,
		Evidence:            arg.Evidence,
		EvidenceFingerprint: arg.EvidenceFingerprint,
		AgentID:             arg.AgentID,
		CreatedAt:           pgtype.Timestamptz{Valid: false},
	}
	s.reports = append(s.reports, row)
	return row, nil
}

func (s *stubTodoIncompleteStore) GetTodoIncompleteReportsByTodo(_ context.Context, todoID int32) ([]db.ZdxTodoIncompleteReport, error) {
	var out []db.ZdxTodoIncompleteReport
	for _, r := range s.reports {
		if r.TodoID == todoID {
			out = append(out, r)
		}
	}
	return out, nil
}

func (s *stubTodoIncompleteStore) AggregateTodoIncompleteReports(_ context.Context, arg db.AggregateTodoIncompleteReportsParams) ([]db.AggregateTodoIncompleteReportsRow, error) {
	type key struct{ reason, fp string }
	counts := map[key]int64{}
	ids := map[key][]int32{}
	suggested := map[key][]byte{}
	for _, r := range s.reports {
		if r.ProjectID != arg.ProjectID {
			continue
		}
		if arg.Reason.Valid && r.Reason != arg.Reason.String {
			continue
		}
		k := key{r.Reason, r.EvidenceFingerprint}
		counts[k]++
		ids[k] = append(ids[k], r.TodoID)
		if len(r.SuggestedNext) > 0 && string(r.SuggestedNext) != "{}" {
			suggested[k] = r.SuggestedNext
		}
	}
	var rows []db.AggregateTodoIncompleteReportsRow
	for k, c := range counts {
		rows = append(rows, db.AggregateTodoIncompleteReportsRow{
			Reason:              k.reason,
			EvidenceFingerprint: k.fp,
			TotalCount:          c,
			AffectedTodoIds:     ids[k],
			SuggestedNext:       suggested[k],
		})
	}
	return rows, nil
}

func (s *stubTodoIncompleteStore) GetTodoByID(_ context.Context, id int32) (db.GetTodoByIDRow, error) {
	if s.todo.ID != id {
		return db.GetTodoByIDRow{}, fmt.Errorf("not found")
	}
	return db.GetTodoByIDRow{
		ID:        s.todo.ID,
		ProjectID: s.todo.ProjectID,
		Key:       s.todo.Key,
		TargetID:  s.todo.TargetID,
	}, nil
}

func (s *stubTodoIncompleteStore) InsertSideEffectIfNew(_ context.Context, arg db.InsertSideEffectIfNewParams) (db.ZdxIncompleteReportSideEffect, error) {
	if s.sideEffects == nil {
		s.sideEffects = map[string]bool{}
	}
	key := fmt.Sprintf("%d:%s:%s:%s", arg.ProjectID, arg.Reason, arg.EvidenceFingerprint, arg.ActionType)
	if s.sideEffects[key] {
		return db.ZdxIncompleteReportSideEffect{}, pgx.ErrNoRows
	}
	s.sideEffects[key] = true
	return db.ZdxIncompleteReportSideEffect{
		ID:                  1,
		ProjectID:           arg.ProjectID,
		Reason:              arg.Reason,
		EvidenceFingerprint: arg.EvidenceFingerprint,
		ActionType:          arg.ActionType,
		Meta:                arg.Meta,
	}, nil
}

func (s *stubTodoIncompleteStore) GetIssueByAnyProject(_ context.Context, id string) (db.ZdxIssue, error) {
	if s.issues != nil {
		if issue, ok := s.issues[id]; ok {
			return issue, nil
		}
	}
	return db.ZdxIssue{}, fmt.Errorf("not found: %s", id)
}

func (s *stubTodoIncompleteStore) AddIssueBlock(_ context.Context, arg db.AddIssueBlockParams) error {
	s.blocks = append(s.blocks, arg)
	return nil
}

func (s *stubTodoIncompleteStore) InsertQuestion(_ context.Context, arg db.InsertQuestionParams) (db.ZdxQuestion, error) {
	s.issueCounter++
	q := db.ZdxQuestion{ID: s.issueCounter, ProjectID: arg.ProjectID, Category: arg.Category, Question: arg.Question}
	s.questions = append(s.questions, q)
	return q, nil
}

func (s *stubTodoIncompleteStore) NextIssueID(_ context.Context) (string, error) {
	s.issueCounter++
	return fmt.Sprintf("IS-%d", s.issueCounter), nil
}

func (s *stubTodoIncompleteStore) CreateIssue(_ context.Context, arg db.CreateIssueParams) (db.ZdxIssue, error) {
	issue := db.ZdxIssue{ID: arg.ID, ProjectID: arg.ProjectID, Title: arg.Title, Context: arg.Context, Priority: arg.Priority, Component: arg.Component, IssueType: arg.IssueType, Status: arg.Status}
	if s.issues == nil {
		s.issues = map[string]db.ZdxIssue{}
	}
	s.issues[arg.ID] = issue
	return issue, nil
}

func (s *stubTodoIncompleteStore) GetTestFixIssue(_ context.Context, arg db.GetTestFixIssueParams) (string, error) {
	if s.testFixIssues == nil {
		return "", pgx.ErrNoRows
	}
	key := fmt.Sprintf("%d:%s:%s", arg.ProjectID, arg.Reason, arg.EvidenceFingerprint)
	if id, ok := s.testFixIssues[key]; ok {
		return id, nil
	}
	return "", pgx.ErrNoRows
}

func (s *stubTodoIncompleteStore) InsertTestFixIssue(_ context.Context, arg db.InsertTestFixIssueParams) error {
	if s.testFixIssues == nil {
		s.testFixIssues = map[string]string{}
	}
	key := fmt.Sprintf("%d:%s:%s", arg.ProjectID, arg.Reason, arg.EvidenceFingerprint)
	if _, ok := s.testFixIssues[key]; !ok {
		s.testFixIssues[key] = arg.IssueID
	}
	return nil
}

func (s *stubTodoIncompleteStore) AddComment(_ context.Context, arg db.AddCommentParams) (db.AddCommentRow, error) {
	s.comments = append(s.comments, arg)
	return db.AddCommentRow{ProjectID: arg.ProjectID, TargetType: arg.TargetType, TargetID: arg.TargetID, Author: arg.Author, Body: arg.Body, AuthorAlias: arg.AuthorAlias}, nil
}

func newTodoIncompleteAPI(t *testing.T, store TodoIncompleteStore) humatest.TestAPI {
	t.Helper()
	_, api := humatest.New(t, huma.DefaultConfig("test", "1.0.0"))
	h := &Handler{Deps: &Deps{TodoIncompleteStore: store}}
	h.registerTodoRoutes(api)
	return api
}

func newTestStore() *stubTodoIncompleteStore {
	return &stubTodoIncompleteStore{
		project: db.ZdxProject{ID: 1, Slug: "proj"},
		todo:    db.GetTodoByKeyRow{ID: 10, ProjectID: 1, Key: "TK-1"},
	}
}

func TestPostIncompleteReport200(t *testing.T) {
	api := newTodoIncompleteAPI(t, newTestStore())
	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "ambiguous_spec",
		"explanation": "the spec is unclear",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "ambiguous_spec") {
		t.Errorf("body missing reason: %s", body)
	}
	if !strings.Contains(body, "evidence_fingerprint") {
		t.Errorf("body missing evidence_fingerprint: %s", body)
	}
}

func TestPostIncompleteReport400InvalidReason(t *testing.T) {
	api := newTodoIncompleteAPI(t, newTestStore())
	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "not_a_valid_reason",
		"explanation": "x",
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", resp.Code, resp.Body)
	}
}

func TestIncompleteReportFingerprintDeterminism(t *testing.T) {
	ev := map[string]string{"file": "main.go", "line": "42"}
	fp1 := incompleteEvidenceFingerprint(ev)
	fp2 := incompleteEvidenceFingerprint(ev)
	if fp1 != fp2 {
		t.Errorf("identical evidence produced different fingerprints: %q vs %q", fp1, fp2)
	}
}

func TestIncompleteReportFingerprintDifferentEvidence(t *testing.T) {
	fp1 := incompleteEvidenceFingerprint(map[string]string{"key": "a"})
	fp2 := incompleteEvidenceFingerprint(map[string]string{"key": "b"})
	if fp1 == fp2 {
		t.Errorf("different evidence produced identical fingerprint: %q", fp1)
	}
}

func TestAggregateIncompleteReports(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	// Two reports with same (reason, evidence) → one group with total_count=2
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "capability_gap", "explanation": "missing tool",
	})
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "capability_gap", "explanation": "missing tool",
	})
	// Different fingerprint → second group
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "capability_gap", "explanation": "other", "evidence": map[string]string{"k": "v"},
	})

	resp := api.Get("/api/dx/incomplete-reports?slug=proj")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "capability_gap") {
		t.Errorf("body missing reason: %s", body)
	}
	if !strings.Contains(body, "affected_todo_ids") {
		t.Errorf("body missing affected_todo_ids: %s", body)
	}
}

func TestAggregateIncompleteReportsSurfacesSuggestedNext(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	// First report has no suggested_next; second adds one. Aggregate should surface it.
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "capability_gap", "explanation": "missing tool",
	})
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":         "capability_gap",
		"explanation":    "missing tool",
		"suggested_next": "open IS-X tracker",
	})

	resp := api.Get("/api/dx/incomplete-reports?slug=proj")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"suggested_next"`) {
		t.Errorf("body missing suggested_next field: %s", body)
	}
	if !strings.Contains(body, "open IS-X tracker") {
		t.Errorf("body missing suggested_next payload: %s", body)
	}
}

func TestAggregateIncompleteReportsReasonFilter(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "capability_gap", "explanation": "x",
	})
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason": "flaky_test", "explanation": "y",
	})

	resp := api.Get("/api/dx/incomplete-reports?slug=proj&reason=capability_gap")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "capability_gap") {
		t.Errorf("missing capability_gap: %s", body)
	}
	if strings.Contains(body, "flaky_test") {
		t.Errorf("filter should exclude flaky_test: %s", body)
	}
}

func TestGetIncompleteReports(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "capability_gap",
		"explanation": "missing tool",
	})
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "flaky_test",
		"explanation": "intermittent",
	})

	resp := api.Get("/api/dx/projects/proj/todos/TK-1/incomplete-reports")
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, "capability_gap") {
		t.Errorf("GET missing first report: %s", body)
	}
	if !strings.Contains(body, "flaky_test") {
		t.Errorf("GET missing second report: %s", body)
	}
}

func TestApplyIncompleteReportSideEffectsBlockOn(t *testing.T) {
	store := newTestStore()
	store.todo.TargetID = "IS-2"
	store.issues = map[string]db.ZdxIssue{"IS-1": {ID: "IS-1", ProjectID: 1}}
	api := newTodoIncompleteAPI(t, store)

	// Seed 2 reports with same (reason, fingerprint) and block-on suggested_next.
	for i := 0; i < 2; i++ {
		api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
			"reason":         "capability_gap",
			"explanation":    "blocked",
			"suggested_next": "block on IS-1",
		})
	}

	resp := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"block_on"`) {
		t.Errorf("missing block_on action: %s", body)
	}
	if len(store.blocks) != 1 {
		t.Errorf("expected 1 issue block, got %d", len(store.blocks))
	}
	if store.blocks[0].IssueID != "IS-2" || store.blocks[0].BlockedByID != "IS-1" {
		t.Errorf("wrong block: %+v", store.blocks[0])
	}

	// Second call — idempotent, 0 applied.
	resp2 := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp2.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp2.Code, resp2.Body)
	}
	if strings.Contains(resp2.Body.String(), `"block_on"`) {
		t.Errorf("second call should return 0 applied, got: %s", resp2.Body)
	}
}

func TestApplyIncompleteReportSideEffectsAskUser(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":         "ambiguous_spec",
		"explanation":    "unclear",
		"suggested_next": "ask user: what is the expected format?",
	})

	resp := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"ask_user"`) {
		t.Errorf("missing ask_user action: %s", body)
	}
	if len(store.questions) != 1 {
		t.Errorf("expected 1 question, got %d", len(store.questions))
	}
	if store.questions[0].Question != "what is the expected format?" {
		t.Errorf("wrong question text: %q", store.questions[0].Question)
	}

	// Idempotent.
	resp2 := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp2.Code != http.StatusOK {
		t.Fatalf("second call status = %d", resp2.Code)
	}
	if strings.Contains(resp2.Body.String(), `"ask_user"`) {
		t.Errorf("second call should return 0 applied")
	}
}

func TestApplyIncompleteReportSideEffectsFileCapability(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":         "capability_gap",
		"explanation":    "no tool",
		"suggested_next": "file capability request: add pdf parsing support",
	})

	resp := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	if !strings.Contains(body, `"file_capability_request"`) {
		t.Errorf("missing file_capability_request action: %s", body)
	}
	if len(store.issues) != 1 {
		t.Errorf("expected 1 issue created, got %d", len(store.issues))
	}
	for _, iss := range store.issues {
		if iss.Title != "add pdf parsing support" {
			t.Errorf("wrong issue title: %q", iss.Title)
		}
	}

	// Idempotent.
	resp2 := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp2.Code != http.StatusOK {
		t.Fatalf("second call status = %d", resp2.Code)
	}
	if strings.Contains(resp2.Body.String(), `"file_capability_request"`) {
		t.Errorf("second call should return 0 applied")
	}
}

func TestApplyIncompleteReportSideEffectsUnstructured(t *testing.T) {
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":         "capability_gap",
		"explanation":    "misc",
		"suggested_next": "do something unrecognized",
	})

	resp := api.Post("/api/dx/incomplete-reports/apply", map[string]any{"slug": "proj"})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	body := resp.Body.String()
	// Unstructured suggested_next → 0 applied.
	if strings.Contains(body, `"action_type"`) {
		t.Errorf("unstructured text should produce 0 applied, got: %s", body)
	}
}

// TK-1617: preexisting_test_failure / flaky_test reports auto-promote to a
// priority-1 test-fix issue (deduped by reason+fingerprint), append comments
// on recurrence, and add a sequencing block from the source todo's issue.

func TestPromoteTestFix_FirstReportCreatesIssueAndBlock(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "")
	store := newTestStore()
	store.todo.IssueRef = "IS-100"
	api := newTodoIncompleteAPI(t, store)

	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "preexisting_test_failure",
		"explanation": "TestFoo was already broken before my change",
		"evidence":    map[string]string{"test_name": "TestFoo", "error_fingerprint": "abc123"},
		"agent_id":    "agent-A",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}

	if len(store.issues) != 1 {
		t.Fatalf("expected 1 test-fix issue, got %d", len(store.issues))
	}
	var iss db.ZdxIssue
	for _, v := range store.issues {
		iss = v
	}
	if iss.Title != "Test failure: TestFoo" {
		t.Errorf("title = %q, want %q", iss.Title, "Test failure: TestFoo")
	}
	if iss.Priority != "1" {
		t.Errorf("priority = %q, want 1", iss.Priority)
	}
	if iss.IssueType != "impl" {
		t.Errorf("issue_type = %q, want impl", iss.IssueType)
	}
	if !strings.Contains(iss.Context, "TestFoo was already broken") {
		t.Errorf("context missing explanation: %q", iss.Context)
	}
	if !strings.Contains(iss.Context, "abc123") {
		t.Errorf("context missing evidence: %q", iss.Context)
	}

	if len(store.blocks) != 1 {
		t.Fatalf("expected 1 block edge, got %d", len(store.blocks))
	}
	if store.blocks[0].IssueID != "IS-100" || store.blocks[0].BlockedByID != iss.ID || store.blocks[0].Kind != "sequencing" {
		t.Errorf("wrong block: %+v (test-fix issue=%s)", store.blocks[0], iss.ID)
	}
}

func TestPromoteTestFix_SecondReportSameFingerprintDedups(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "")
	store := newTestStore()
	store.todo.IssueRef = "IS-100"
	api := newTodoIncompleteAPI(t, store)

	post := func() {
		resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
			"reason":      "preexisting_test_failure",
			"explanation": "broken",
			"evidence":    map[string]string{"test_name": "TestFoo"},
			"agent_id":    "agent-A",
		})
		if resp.Code != http.StatusOK {
			t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
		}
	}
	post()
	post()

	if len(store.issues) != 1 {
		t.Errorf("expected exactly 1 test-fix issue (dedup), got %d", len(store.issues))
	}
	if len(store.reports) != 2 {
		t.Errorf("expected 2 stored reports, got %d", len(store.reports))
	}
	if len(store.comments) != 1 {
		t.Errorf("expected 1 recurrence comment, got %d", len(store.comments))
	}
	if len(store.comments) > 0 && store.comments[0].TargetType != "issue" {
		t.Errorf("comment target_type = %q, want issue", store.comments[0].TargetType)
	}
}

func TestPromoteTestFix_DifferentReasonSameFingerprint_TwoIssues(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "")
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "preexisting_test_failure",
		"explanation": "x",
		"evidence":    map[string]string{"test_name": "TestFoo"},
	})
	api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "flaky_test",
		"explanation": "y",
		"evidence":    map[string]string{"test_name": "TestFoo"},
	})

	if len(store.issues) != 2 {
		t.Errorf("expected 2 distinct test-fix issues (reason is part of dedup key), got %d", len(store.issues))
	}
}

func TestPromoteTestFix_NonTestReason_NoSideEffect(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "")
	store := newTestStore()
	store.todo.IssueRef = "IS-100"
	api := newTodoIncompleteAPI(t, store)

	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "capability_gap",
		"explanation": "missing tool",
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	if len(store.issues) != 0 {
		t.Errorf("expected no test-fix issue for capability_gap, got %d", len(store.issues))
	}
	if len(store.blocks) != 0 {
		t.Errorf("expected no block edge, got %d", len(store.blocks))
	}
}

func TestPromoteTestFix_FeatureFlagOff_Skips(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "off")
	store := newTestStore()
	store.todo.IssueRef = "IS-100"
	api := newTodoIncompleteAPI(t, store)

	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "preexisting_test_failure",
		"explanation": "x",
		"evidence":    map[string]string{"test_name": "TestFoo"},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	if len(store.reports) != 1 {
		t.Errorf("report should still be stored, got %d", len(store.reports))
	}
	if len(store.issues) != 0 {
		t.Errorf("flag off → no test-fix issue, got %d", len(store.issues))
	}
	if len(store.blocks) != 0 {
		t.Errorf("flag off → no block edge, got %d", len(store.blocks))
	}
}

func TestPromoteTestFix_MissingTestNameFallsBack(t *testing.T) {
	t.Setenv("ZDX_AUTO_PROMOTE_TEST_FIX", "")
	store := newTestStore()
	api := newTodoIncompleteAPI(t, store)

	resp := api.Post("/api/dx/projects/proj/todos/TK-1/incomplete-reports", map[string]any{
		"reason":      "flaky_test",
		"explanation": "intermittent",
		"evidence":    map[string]string{"build_id": "42"},
	})
	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, body: %s", resp.Code, resp.Body)
	}
	if len(store.issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(store.issues))
	}
	for _, iss := range store.issues {
		if !strings.Contains(iss.Title, "unknown test") {
			t.Errorf("title should fall back when test_name missing, got %q", iss.Title)
		}
	}
}
