package e2e

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

// TestDemoMultiBrowser_Concurrent demonstrates two independent browser
// contexts recording a collaboration session. Each "user" drives its own
// Playwright context (isolated storage, separate console buffers, separate
// video) so the demo shows two simultaneous viewpoints. The output lives
// under .zdx/demo/video/multi/<user>/ and is uploaded as two separate demo
// artifacts.
func TestDemoMultiBrowser_Concurrent(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	baseDir := filepath.Join(root, ".zdx", "demo", "video", "multi")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatalf("mkdir base: %v", err)
	}

	if err := pw.Install(&pw.RunOptions{Browsers: []string{"chromium"}}); err != nil {
		t.Fatalf("install playwright: %v", err)
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

	users := []string{"alice", "bob"}
	errs := make(chan error, len(users))
	caps := make(map[string]*consoleCapture, len(users))
	dirs := make(map[string]string, len(users))
	for _, u := range users {
		userDir := filepath.Join(baseDir, u)
		if err := os.MkdirAll(userDir, 0755); err != nil {
			t.Fatalf("mkdir %s: %v", u, err)
		}
		dirs[u] = userDir
		caps[u] = &consoleCapture{}
	}

	for _, u := range users {
		user := u
		go func() {
			ctx, err := browser.NewContext(pw.BrowserNewContextOptions{
				RecordVideo: &pw.RecordVideo{
					Dir:  dirs[user],
					Size: &pw.Size{Width: 1280, Height: 720},
				},
				UserAgent: pw.String(fmt.Sprintf("zdx-demo/%s", user)),
			})
			if err != nil {
				errs <- fmt.Errorf("%s: new context: %w", user, err)
				return
			}
			page, err := ctx.NewPage()
			if err != nil {
				errs <- fmt.Errorf("%s: new page: %w", user, err)
				return
			}
			page.OnConsole(caps[user].handle)
			resp, err := page.Goto(srv.URL+"/api/health", pw.PageGotoOptions{
				WaitUntil: pw.WaitUntilStateNetworkidle,
			})
			if err != nil {
				errs <- fmt.Errorf("%s: goto: %w", user, err)
				return
			}
			if resp.Status() != 200 {
				errs <- fmt.Errorf("%s: status %d", user, resp.Status())
				return
			}
			time.Sleep(500 * time.Millisecond)
			page.Close()
			ctx.Close()
			errs <- nil
		}()
	}
	for range users {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}

	for _, u := range users {
		entries, err := os.ReadDir(dirs[u])
		if err != nil {
			t.Errorf("%s: read dir: %v", u, err)
			continue
		}
		var webmBase string
		for _, e := range entries {
			if filepath.Ext(e.Name()) == ".webm" {
				info, _ := e.Info()
				if info != nil && info.Size() > 0 {
					webmBase = strings.TrimSuffix(e.Name(), ".webm")
					t.Logf("%s: video %s (%d bytes)", u, e.Name(), info.Size())
				}
			}
		}
		if webmBase == "" {
			t.Errorf("%s: no .webm produced", u)
			webmBase = u
		}
		logPath := filepath.Join(dirs[u], webmBase+".console.log")
		if err := caps[u].flush(logPath); err != nil {
			t.Logf("%s: flush log: %v", u, err)
		}
	}

	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_multi_browser_test.go", Note: "multi-browser demo source"},
		{FilePath: "internal/server/handlers/handlers_auth.go", Note: "exercised /api/health handler"},
	})
}
