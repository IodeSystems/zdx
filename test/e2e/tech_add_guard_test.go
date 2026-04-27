package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// IS-487: dx todo tech add must refuse to attach a new task to a closed
// issue, an unknown issue, or an unknown feature, instead of silently
// accepting the link and letting the cascade-close machinery delete the
// task on the next solo cycle.

func TestTechAddRejectsClosedIssue(t *testing.T) {
	slug := "e2e-tech-add-closed-issue"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Closed Issue"}, nil)

	var iss struct {
		ID int32 `json:"id"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/add",
		map[string]any{"slug": slug, "title": "to be closed", "auto_ready": true}, &iss))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/todo/issue/close",
		map[string]any{"slug": slug, "id": iss.ID, "reason": "done"}, nil))
	closedID := fmt.Sprintf("IS-%d", iss.ID)

	resp := apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "should be refused",
			"issue":      closedID,
			"auto_ready": true,
		}, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("tech add against closed issue: want 400, got %d", resp.StatusCode)
	}
	body := readBody(t, resp)
	if !strings.Contains(body, closedID) || !strings.Contains(body, "closed") {
		t.Errorf("error body should name the closed issue and its status; got %s", body)
	}
}

func TestTechAddRejectsUnknownIssue(t *testing.T) {
	slug := "e2e-tech-add-unknown-issue"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Unknown Issue"}, nil)

	resp := apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "should be refused",
			"issue":      "IS-99999",
			"auto_ready": true,
		}, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("tech add against unknown issue: want 404, got %d", resp.StatusCode)
	}
}

func TestTechAddRejectsUnknownFeature(t *testing.T) {
	slug := "e2e-tech-add-unknown-feature"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "Tech Add Unknown Feature"}, nil)

	resp := apiDo(t, http.MethodPost, "/api/dx/todo/tech/add",
		map[string]any{
			"slug":       slug,
			"text":       "should be refused",
			"feature":    "no-such-feature",
			"auto_ready": true,
		}, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("tech add against unknown feature: want 404, got %d", resp.StatusCode)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var buf [4096]byte
	n, _ := resp.Body.Read(buf[:])
	return string(buf[:n])
}
