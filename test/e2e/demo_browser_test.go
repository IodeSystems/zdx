//go:build demo

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	pw "github.com/playwright-community/playwright-go"
)

func TestDemoBrowser_IssueFlow(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatalf("find root: %v", err)
	}
	videoDir := filepath.Join(root, ".zdx", "demo", "video")
	if err := os.MkdirAll(videoDir, 0755); err != nil {
		t.Fatalf("mkdir video dir: %v", err)
	}

	runErr := pw.Install(&pw.RunOptions{Browsers: []string{"chromium"}})
	if runErr != nil {
		t.Fatalf("install playwright browsers: %v", runErr)
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

	ctx, err := browser.NewContext(pw.BrowserNewContextOptions{
		RecordVideo: &pw.RecordVideo{
			Dir: videoDir,
			Size: &pw.Size{
				Width:  1280,
				Height: 720,
			},
		},
	})
	if err != nil {
		t.Fatalf("new context: %v", err)
	}

	page, err := ctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}

	resp, err := page.Goto(srv.URL+"/api/health", pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateNetworkidle,
	})
	if err != nil {
		t.Fatalf("goto health: %v", err)
	}
	if resp.Status() != 200 {
		t.Fatalf("health status: want 200 got %d", resp.Status())
	}

	// Give the recorder time to capture frames.
	time.Sleep(500 * time.Millisecond)

	// Close page then context to flush video.
	page.Close()
	ctx.Close()

	entries, err := os.ReadDir(videoDir)
	if err != nil {
		t.Fatalf("read video dir: %v", err)
	}
	found := false
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".webm" {
			info, _ := e.Info()
			if info != nil && info.Size() > 0 {
				found = true
				t.Logf("video captured: %s (%d bytes)", e.Name(), info.Size())
			}
		}
	}
	if !found {
		t.Error("no .webm video file found in .zdx/demo/video/")
	}
}
