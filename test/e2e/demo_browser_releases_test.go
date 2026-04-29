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

// TestDemoBrowser_ReleaseIndexGroupsByVersion demonstrates spec 155:
// Given a project with environments, when the Release index is visited,
// then it shows all environments grouped by version branch with current
// deploy SHA, branch, deployed-at, and URL.
func TestDemoBrowser_ReleaseIndexGroupsByVersion(t *testing.T) {
	requiresUI(t)

	const slug = "demo-release-index"
	mustOK(t, apiDo(t, http.MethodPost, "/api/project",
		map[string]any{"slug": slug, "name": "Demo Release Index"}, nil))

	// Seed two environments on distinct version branches.
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{
			"name":           "staging",
			"release_branch": "release/v1.x",
			"url":            "https://staging.example.com",
		}, nil))
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments", slug),
		map[string]any{
			"name":           "production",
			"release_branch": "release/v2.x",
			"url":            "https://prod.example.com",
		}, nil))

	// Record a deploy on each environment so SHA, branch, and deployed-at are populated.
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments/%s/deploys", slug, "staging"),
		map[string]any{
			"build_sha":    "aaaa1111bbbb2222",
			"build_branch": "release/v1.x",
		}, nil))
	mustOK(t, apiDo(t, http.MethodPost,
		fmt.Sprintf("/api/dx/projects/%s/environments/%s/deploys", slug, "production"),
		map[string]any{
			"build_sha":    "cccc3333dddd4444",
			"build_branch": "release/v2.x",
		}, nil))

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

	releasesURL := fmt.Sprintf("%s/project/%s/releases/", srv.URL, slug)
	if _, err := page.Goto(releasesURL, pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateDomcontentloaded,
	}); err != nil {
		t.Fatalf("goto releases index: %v", err)
	}

	timeout := float64(15000)

	// Wait for at least one group header to appear.
	groupHeader := page.GetByText("release/v1.x").First()
	if err := groupHeader.WaitFor(pw.LocatorWaitForOptions{
		State:   pw.WaitForSelectorStateVisible,
		Timeout: &timeout,
	}); err != nil {
		t.Fatalf("release/v1.x group header not visible within 15s: %v", err)
	}

	// Assert both version branch group headers are visible.
	if ok, _ := page.GetByText("release/v1.x").First().IsVisible(); !ok {
		t.Error("group header for release/v1.x not visible")
	}
	if ok, _ := page.GetByText("release/v2.x").First().IsVisible(); !ok {
		t.Error("group header for release/v2.x not visible")
	}

	// Assert each environment row is present.
	if ok, _ := page.GetByText("staging").First().IsVisible(); !ok {
		t.Error("staging environment row not visible")
	}
	if ok, _ := page.GetByText("production").First().IsVisible(); !ok {
		t.Error("production environment row not visible")
	}

	// Assert short SHA is shown for staging (first 8 chars of aaaa1111bbbb2222).
	shortSHA := "aaaa1111"
	if ok, _ := page.GetByText(shortSHA).IsVisible(); !ok {
		t.Errorf("short SHA %q not visible for staging", shortSHA)
	}

	// Assert short SHA is shown for production (first 8 chars of cccc3333dddd4444).
	shortSHAProd := "cccc3333"
	if ok, _ := page.GetByText(shortSHAProd).IsVisible(); !ok {
		t.Errorf("short SHA %q not visible for production", shortSHAProd)
	}

	// Assert URLs are visible.
	if ok, _ := page.GetByText("https://staging.example.com").IsVisible(); !ok {
		t.Error("staging URL not visible")
	}
	if ok, _ := page.GetByText("https://prod.example.com").IsVisible(); !ok {
		t.Error("production URL not visible")
	}

	// Assert deployed-at is shown (not "—") — both envs were just deployed.
	stagingDeployedAt := page.Locator("[data-testid='env-deployed-at-staging']")
	if ok, _ := stagingDeployedAt.IsVisible(); !ok {
		t.Error("staging deployed-at cell not visible")
	}
	stagingText, _ := stagingDeployedAt.TextContent()
	if stagingText == "—" {
		t.Error("staging deployed-at shows dash; expected relative time")
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
		{FilePath: "test/e2e/demo_browser_releases_test.go", Note: "browser demo source (spec 155)"},
		{FilePath: "ui/src/routes/project/$slug/releases/index.tsx", Note: "Release index route — envs grouped by version branch"},
		{FilePath: "internal/server/handlers/handlers_environments.go", Note: "list-environments handler"},
	})
}
