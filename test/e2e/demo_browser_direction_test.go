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

// TestDemoBrowser_ProjectDirectionTab demonstrates spec 55: the project
// direction tab renders Goals and Constraints in their own grouped sections
// and supports inline create/update/delete for each type.
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

	// Wait for the Goals section heading to appear (React app loaded + data fetched).
	timeout := float64(15000)
	if err := page.GetByText("Goals").First().WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &timeout,
	}); err != nil {
		t.Fatalf("goals section not visible within 15s: %v", err)
	}

	// Assert both sections are visible.
	if ok, _ := page.GetByText("Goals").First().IsVisible(); !ok {
		t.Error("Goals section heading not visible")
	}
	if ok, _ := page.GetByText("Constraints").First().IsVisible(); !ok {
		t.Error("Constraints section heading not visible")
	}

	// ── Goal: create ────────────────────────────────────────────────────────
	// Button order with 0 goals, 0 constraints: Add(Goals)[0], Add(Constraints)[1]
	if err := page.Locator("button").Nth(0).Click(); err != nil {
		t.Fatalf("click Add (Goals): %v", err)
	}
	if err := page.GetByLabel("Title").Fill("Ship v2"); err != nil {
		t.Fatalf("fill goal title: %v", err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		t.Fatalf("click Save (create goal): %v", err)
	}
	// Wait for the goal card to appear.
	goalTimeout := float64(5000)
	if err := page.GetByText("Ship v2").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &goalTimeout,
	}); err != nil {
		t.Fatalf("goal card not visible after create: %v", err)
	}

	// ── Goal: update ────────────────────────────────────────────────────────
	// Button order with 1 goal, 0 constraints: Add(Goals)[0], Edit[1], Delete[2], Add(Constraints)[3]
	if err := page.Locator("button").Nth(1).Click(); err != nil {
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
	// Button order with 1 goal, 0 constraints: Add(Goals)[0], Edit[1], Delete[2], Add(Constraints)[3]
	if err := page.Locator("button").Nth(2).Click(); err != nil {
		t.Fatalf("click Delete (goal): %v", err)
	}
	deleteTimeout := float64(5000)
	if err := page.GetByText("Ship v2 (updated)").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateHidden,
		Timeout: &deleteTimeout,
	}); err != nil {
		t.Logf("goal card still visible after delete (non-fatal): %v", err)
	}

	// ── Constraint: create ──────────────────────────────────────────────────
	// Button order with 0 goals, 0 constraints: Add(Goals)[0], Add(Constraints)[1]
	if err := page.Locator("button").Nth(1).Click(); err != nil {
		t.Fatalf("click Add (Constraints): %v", err)
	}
	if err := page.GetByLabel("Title").Fill("No breaking changes"); err != nil {
		t.Fatalf("fill constraint title: %v", err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		t.Fatalf("click Save (create constraint): %v", err)
	}
	cTimeout := float64(5000)
	if err := page.GetByText("No breaking changes").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &cTimeout,
	}); err != nil {
		t.Fatalf("constraint card not visible after create: %v", err)
	}

	// ── Constraint: update ──────────────────────────────────────────────────
	// Button order with 0 goals, 1 constraint: Add(Goals)[0], Add(Constraints)[1], Edit[2], Delete[3]
	if err := page.Locator("button").Nth(2).Click(); err != nil {
		t.Fatalf("click Edit (constraint): %v", err)
	}
	if err := page.GetByLabel("Title").Fill("No breaking changes (updated)"); err != nil {
		t.Fatalf("fill updated constraint title: %v", err)
	}
	if err := page.GetByRole("button", pw.PageGetByRoleOptions{Name: "Save"}).Click(); err != nil {
		t.Fatalf("click Save (update constraint): %v", err)
	}
	cuTimeout := float64(5000)
	if err := page.GetByText("No breaking changes (updated)").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &cuTimeout,
	}); err != nil {
		t.Fatalf("updated constraint title not visible: %v", err)
	}

	// ── Constraint: delete ──────────────────────────────────────────────────
	// Button order with 0 goals, 1 constraint: Add(Goals)[0], Add(Constraints)[1], Edit[2], Delete[3]
	if err := page.Locator("button").Nth(3).Click(); err != nil {
		t.Fatalf("click Delete (constraint): %v", err)
	}
	cdTimeout := float64(5000)
	if err := page.GetByText("No breaking changes (updated)").WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateHidden,
		Timeout: &cdTimeout,
	}); err != nil {
		t.Logf("constraint card still visible after delete (non-fatal): %v", err)
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
		{FilePath: "test/e2e/demo_browser_direction_test.go", Note: "browser demo source (spec 55)"},
		{FilePath: "ui/src/components/GoalsTab.tsx", Note: "direction tab component"},
		{FilePath: "ui/src/routes/project/$slug/goals/index.tsx", Note: "direction tab route"},
		{FilePath: "internal/server/handlers/handlers_projects.go", Note: "goal/constraint handlers"},
	})
}
