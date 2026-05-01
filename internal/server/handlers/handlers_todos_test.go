package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/humatest"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// stubTodoIncompleteStore implements TodoIncompleteStore for unit tests.
type stubTodoIncompleteStore struct {
	project db.ZdxProject
	todo    db.GetTodoByKeyRow
	reports []db.ZdxTodoIncompleteReport
	nextID  int64
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
