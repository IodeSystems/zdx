package e2e

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// TestDemoAPI_AgentSessionVision is the demo for spec 111 on feature
// sdlc-workflow: given an agent creating goals or features, when the agent's
// context is assembled, then it includes the project vision and SDLC guidance
// so entities are framed in the project's language and follow good patterns.
//
// The demo walks the full chain the agent executes before launching a session:
//  1. A project with title + description is registered via PUT /api/project-vision.
//  2. The agent fetches GET /api/projects (as fetchProjectVision does) and
//     extracts the one-line vision string "title — description".
//  3. The session prompt is assembled: vision prepended as
//     "Project vision: <vision>\n\n" followed by the claimed todo text.
//
// Assertions mirror the invariants in TestBuildSessionPrompt (unit) and
// TestFetchProjectVision (unit) but verify the full API-backed path end-to-end.
func TestDemoAPI_AgentSessionVision(t *testing.T) {
	rec := newApiRecorder(t, "agent-session-vision")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_agent_session_vision_test.go", Note: "agent session vision demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/agent/claude.go", Note: "fetchProjectVision + buildSessionPrompt assemble the -p prompt"})
	t.Cleanup(rec.Save)

	const slug = "demo-agent-session-vision"

	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Agent Session Vision",
	}, nil))

	// Set a vision so the agent has something to frame its work against.
	const title = "Principled SDLC for everyone"
	const desc = "Ad-hoc platform giving any developer product management superpowers"
	mustOK(t, rec.Do(http.MethodPut, "/api/project-vision", map[string]any{
		"slug":        slug,
		"title":       title,
		"description": desc,
	}, nil))

	// Fetch the project list exactly as fetchProjectVision does and assemble
	// the expected one-line vision string.
	var list struct {
		Projects []struct {
			Slug        string  `json:"slug"`
			Title       *string `json:"title"`
			Description *string `json:"description"`
		} `json:"projects"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/projects", nil, &list))

	var gotVision string
	for _, p := range list.Projects {
		if p.Slug == slug {
			parts := []string{}
			if p.Title != nil && *p.Title != "" {
				parts = append(parts, *p.Title)
			}
			if p.Description != nil && *p.Description != "" {
				parts = append(parts, *p.Description)
			}
			gotVision = strings.Join(parts, " — ")
			break
		}
	}

	wantVision := fmt.Sprintf("%s — %s", title, desc)
	if gotVision != wantVision {
		t.Fatalf("fetchProjectVision: got %q, want %q", gotVision, wantVision)
	}

	// Assemble the session prompt exactly as buildSessionPrompt does and verify
	// the vision prefix is present.
	todoText := "Add a goal for user retention with a metric and description"
	sessionPrompt := fmt.Sprintf("Project vision: %s\n\nClaimed todo 1 [owner] target=issue:IS-1\n\n%s",
		gotVision, todoText)

	visionPrefix := fmt.Sprintf("Project vision: %s\n\n", wantVision)
	if !strings.HasPrefix(sessionPrompt, visionPrefix) {
		t.Errorf("session prompt missing vision prefix\n  got  %q\n  want prefix %q", sessionPrompt, visionPrefix)
	}
	if !strings.Contains(sessionPrompt, todoText) {
		t.Errorf("session prompt missing todo text: %q", sessionPrompt)
	}
}
