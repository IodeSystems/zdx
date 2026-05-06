package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	pw "github.com/playwright-community/playwright-go"
)

// TestDemoBrowser_AgentsPoolPanel demonstrates GAPD phase 2 (docs/plan.md):
// the top-level /agents panel renders project-scoped + global-pool rows,
// hides the pin button on project-scoped rows (scope-immutable caption),
// and pins / unpins originally-global agents end-to-end.
//
// Replaces the manual UI walkthrough that previously gated phase-2 sign-off.
func TestDemoBrowser_AgentsPoolPanel(t *testing.T) {
	requiresUI(t)
	if srv.DSN == "" {
		t.Skip("srv.DSN unavailable; needs direct DB access to seed global-pool agent")
	}

	const slug = "demo-agents-pool"
	const projectName = "Demo Agents Pool"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]any{"slug": slug, "name": projectName}, nil))

	projectAgentID := registerAgent(t, slug, "demo-pool-proj", "demo-session-proj", "")

	// No public REST endpoint registers a global-pool agent — that path is
	// the WS handshake. For a deterministic seed, INSERT directly. Mirrors
	// setAgentDisconnectAt in task_reclaim_disconnected_agent_test.go.
	const globalAgentID = "demo-pool-glob"
	{
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, srv.DSN)
		if err != nil {
			t.Fatalf("pgx connect: %v", err)
		}
		defer conn.Close(ctx)
		if _, err := conn.Exec(ctx, `
			INSERT INTO zdx_agents (id, project_id, session_id, status, originally_global)
			VALUES ($1, NULL, '', 'active', true)
			ON CONFLICT (id) DO UPDATE
			   SET project_id = NULL, status = 'active', originally_global = true`,
			globalAgentID); err != nil {
			t.Fatalf("seed global agent: %v", err)
		}
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		conn, err := pgx.Connect(ctx, srv.DSN)
		if err != nil {
			return
		}
		defer conn.Close(ctx)
		conn.Exec(ctx, `DELETE FROM zdx_agents WHERE id IN ($1, $2)`,
			globalAgentID, projectAgentID)
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	videoDir := filepath.Join(root, ".zdx", "demo", "video")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatalf("mkdir video dir: %v", err)
	}

	if err := pw.Install(&pw.RunOptions{Browsers: []string{"chromium"}}); err != nil {
		t.Fatalf("install playwright browsers: %v", err)
	}
	pwi, err := pw.Run()
	if err != nil {
		t.Fatalf("start playwright: %v", err)
	}
	t.Cleanup(func() { pwi.Stop() })

	browser, err := pwi.Chromium.Launch()
	if err != nil {
		t.Fatalf("launch chromium: %v", err)
	}
	t.Cleanup(func() { browser.Close() })

	bctx, err := browser.NewContext(pw.BrowserNewContextOptions{
		RecordVideo: &pw.RecordVideo{
			Dir:  videoDir,
			Size: &pw.Size{Width: 1280, Height: 720},
		},
	})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}

	if err := bctx.AddInitScript(pw.Script{
		Content: pw.String(fmt.Sprintf(`localStorage.setItem('zdx_api_token', %q)`, srv.AdminToken)),
	}); err != nil {
		t.Fatalf("add init script: %v", err)
	}

	page, err := bctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	cap := &consoleCapture{}
	page.OnConsole(cap.handle)

	timeout := float64(15000)

	if _, err := page.Goto(srv.URL+"/agents", pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateDomcontentloaded,
	}); err != nil {
		t.Fatalf("goto /agents: %v", err)
	}

	// Both rows must be visible.
	if err := page.GetByText(globalAgentID).First().WaitFor(pw.LocatorWaitForOptions{
		State: pw.WaitForSelectorStateVisible, Timeout: &timeout,
	}); err != nil {
		t.Fatalf("global agent row not visible within 15s: %v", err)
	}
	if ok, _ := page.GetByText(projectAgentID).First().IsVisible(); !ok {
		t.Error("project-scoped agent row not visible")
	}

	// Project-scoped row: pin column shows the scope-immutable caption (no pin button).
	if ok, _ := page.GetByText("scope-immutable").First().IsVisible(); !ok {
		t.Error("scope-immutable caption not visible on project-scoped row")
	}

	// Global row: scope cell renders the 'global' chip (exact match — the
	// page subtitle also contains the substring "global pool").
	exact := pw.Bool(true)
	if ok, _ := page.GetByText("global", pw.PageGetByTextOptions{Exact: exact}).First().IsVisible(); !ok {
		t.Error("'global' chip not visible for global agent")
	}

	// Pin: click pin icon → choose project → confirm.
	// MUI's Tooltip wraps each IconButton in a <span> with aria-label set to
	// the tooltip's title (the button itself has no accessible name, and the
	// prod build strips Material-UI icon data-testids — so neither
	// GetByRole("button", {Name: …}) nor a `svg[data-testid="LinkIcon"]`
	// selector matches). Locate the span by aria-label, descend to the button.
	pinBtn := page.Locator(`span[aria-label="Pin to a project"] button`).First()
	if err := pinBtn.Click(); err != nil {
		t.Fatalf("click pin icon button: %v", err)
	}
	if err := page.GetByRole("combobox").First().Click(); err != nil {
		t.Fatalf("open project select: %v", err)
	}
	if err := page.GetByRole("option", pw.PageGetByRoleOptions{Name: projectName}).Click(); err != nil {
		t.Fatalf("pick project option %q: %v", projectName, err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Pin", Exact: exact}).Click(); err != nil {
		t.Fatalf("click Pin confirm button: %v", err)
	}

	// After pin, the panel auto-refreshes (5s) and the row's scope cell shows
	// the project name with a 'pinned' chip beside it.
	pinned := page.GetByText("pinned").First()
	if err := pinned.WaitFor(pw.LocatorWaitForOptions{
		State: pw.WaitForSelectorStateVisible, Timeout: &timeout,
	}); err != nil {
		t.Errorf("'pinned' chip not visible after assign: %v", err)
	}
	if ok, _ := page.GetByText(projectName).First().IsVisible(); !ok {
		t.Error("project name not visible in scope cell after assign")
	}

	// Unpin: click the unpin (link-off) icon, wait for the 'global' chip to
	// re-appear.
	unpinBtn := page.Locator(`span[aria-label="Unpin from project (back to global pool)"] button`).First()
	if err := unpinBtn.Click(); err != nil {
		t.Fatalf("click unpin icon button: %v", err)
	}
	globalChip := page.GetByText("global", pw.PageGetByTextOptions{Exact: exact}).First()
	if err := globalChip.WaitFor(pw.LocatorWaitForOptions{
		State: pw.WaitForSelectorStateVisible, Timeout: &timeout,
	}); err != nil {
		t.Errorf("'global' chip not visible after unassign: %v", err)
	}

	time.Sleep(500 * time.Millisecond)
	page.Close()
	bctx.Close()

	entries, err := os.ReadDir(videoDir)
	if err != nil {
		t.Fatalf("read video dir: %v", err)
	}
	found := false
	var webmBase string
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".webm" {
			info, _ := e.Info()
			if info != nil && info.Size() > 0 {
				found = true
				webmBase = strings.TrimSuffix(e.Name(), ".webm")
				t.Logf("video captured: %s (%d bytes)", e.Name(), info.Size())
			}
		}
	}
	if !found {
		t.Error("no .webm video file found in .zdx/demo/video/")
	}
	logPath := filepath.Join(videoDir, webmBase+".console.log")
	if webmBase == "" {
		logPath = filepath.Join(videoDir, t.Name()+".console.log")
	}
	if err := cap.flush(logPath); err != nil {
		t.Logf("flush console log: %v", err)
	}

	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_browser_agents_pool_test.go", Note: "browser demo source (GAPD phase 2)"},
		{FilePath: "ui/src/routes/agents.tsx", Note: "Agents pool panel route + PinControls"},
		{FilePath: "internal/server/handlers/handlers_agents.go", Note: "GET /api/agents + POST/DELETE /assign"},
	})
}
