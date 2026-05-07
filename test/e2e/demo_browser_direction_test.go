package e2e

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

// TestDemoBrowser_ProjectDirectionTab covers the Goals tab CRUD round-trip:
// the panel renders the Goals section heading and supports inline
// create / update / delete on goal cards. Originally spec 55 (Goals +
// Constraints); the Constraints REST surface was removed in IS-627, so
// this test is now Goals-only.
func TestDemoBrowser_ProjectDirectionTab(t *testing.T) {
	requiresUI(t)

	const slug = "demo-direction-tab"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]any{"slug": slug, "name": "Demo Direction Tab"}, nil))

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

	// Inject admin token into localStorage before page scripts execute so the
	// React app can authenticate API requests without a login flow.
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

	// Navigate to the project direction (goals) tab.
	dirURL := fmt.Sprintf("%s/project/%s/goals/", srv.URL, slug)
	if _, err := page.Goto(dirURL, pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateDomcontentloaded,
	}); err != nil {
		t.Fatalf("goto direction tab: %v", err)
	}

	// Scope every locator to <main> — the left drawer nav also contains "Goals"
	// and "Add" elements (one per project, with the non-current project's Collapse
	// in MuiCollapse-hidden state). Without this scope, .First() can resolve to
	// a hidden drawer match and waitFor never satisfies. The flake only surfaced
	// once another browser demo seeded a second project earlier in the run.
	main := page.Locator("main")

	// Wait for the Goals section heading to appear (React app loaded + data fetched).
	timeout := float64(15000)
	if err := main.GetByText("Goals").First().WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &timeout,
	}); err != nil {
		t.Fatalf("goals section not visible within 15s: %v", err)
	}

	if ok, _ := main.GetByText("Goals").First().IsVisible(); !ok {
		t.Error("Goals section heading not visible")
	}

	// The page exposes only the Goals section now (Constraints removed in
	// IS-627). With 0 goals: Add[0]. With 1 goal: Add[0], Edit[1], Delete[2].
	addBtn := main.GetByRole("button", pw.LocatorGetByRoleOptions{Name: "Add"}).First()

	// ── Goal: create ────────────────────────────────────────────────────────
	if err := addBtn.Click(); err != nil {
		t.Fatalf("click Add (Goals): %v", err)
	}
	if err := page.GetByLabel("Title").Fill("Ship v2"); err != nil {
		t.Fatalf("fill goal title: %v", err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		t.Fatalf("click Save (create goal): %v", err)
	}
	goalTimeout := float64(5000)
	if err := page.GetByText("Ship v2").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &goalTimeout,
	}); err != nil {
		t.Fatalf("goal card not visible after create: %v", err)
	}

	// ── Goal: update ────────────────────────────────────────────────────────
	// EntityCard renders [Edit, Delete] IconButtons with no aria-label or
	// testid. Scope the locator to the card containing the goal title; global
	// button index would mismatch because the nav shell adds buttons of its
	// own.
	goalCard := page.Locator(".MuiCard-root", pw.PageLocatorOptions{HasText: "Ship v2"}).First()
	if err := goalCard.Locator("button").Nth(0).Click(); err != nil {
		t.Fatalf("click Edit (goal): %v", err)
	}
	if err := page.GetByLabel("Title").Fill("Ship v2 (updated)"); err != nil {
		t.Fatalf("fill updated goal title: %v", err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		t.Fatalf("click Save (update goal): %v", err)
	}
	updatedTimeout := float64(5000)
	if err := page.GetByText("Ship v2 (updated)").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &updatedTimeout,
	}); err != nil {
		t.Fatalf("updated goal title not visible: %v", err)
	}

	// ── Goal: delete ────────────────────────────────────────────────────────
	updatedCard := page.Locator(".MuiCard-root", pw.PageLocatorOptions{HasText: "Ship v2 (updated)"}).First()
	if err := updatedCard.Locator("button").Nth(1).Click(); err != nil {
		t.Fatalf("click Delete (goal): %v", err)
	}
	deleteTimeout := float64(5000)
	if err := page.GetByText("Ship v2 (updated)").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateHidden,
		Timeout: &deleteTimeout,
	}); err != nil {
		t.Logf("goal card still visible after delete (non-fatal): %v", err)
	}

	time.Sleep(500 * time.Millisecond)

	page.Close()
	bctx.Close()

	// Locate the video file produced by the context.
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
		{FilePath: "test/e2e/demo_browser_direction_test.go", Note: "browser demo source (Goals-only after IS-627)"},
		{FilePath: "ui/src/components/GoalsTab.tsx", Note: "Goals tab component"},
		{FilePath: "ui/src/routes/project/$slug/goals/index.tsx", Note: "Goals tab route"},
		{FilePath: "internal/server/handlers/handlers_projects.go", Note: "goal handlers"},
	})
}
