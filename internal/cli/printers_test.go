package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli/clitypes"
)

// captureStdout redirects os.Stdout for the duration of fn and returns its output.
func captureStdout(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestPrintFeatureItem_TierOutput(t *testing.T) {
	f := clitypes.FeatureItem{
		Name: "my-feature",
		Specs: []clitypes.SpecItem{
			{ID: 10, Importance: "must", Description: "must spec implemented", GreenDemos: 1, TotalDemos: 1},
			{ID: 11, Importance: "must", Description: "must spec planned", GreenDemos: 0, TotalDemos: 0},
			{ID: 20, Importance: "should", Description: "should spec implemented", GreenDemos: 2, TotalDemos: 2},
			{ID: 30, Importance: "nice-to-have", Description: "nice spec planned", GreenDemos: 0, TotalDemos: 0},
		},
	}

	out := captureStdout(func() { PrintFeatureItem(f) })

	// Ship-readiness: one must is blocking
	if !strings.Contains(out, "1 must(s) blocking ship") {
		t.Errorf("expected ship-blocking line; got:\n%s", out)
	}
	if !strings.Contains(out, "SP-11") {
		t.Errorf("expected SP-11 in blocking list; got:\n%s", out)
	}

	// [I] marker for implemented specs, [P] for planned
	if !strings.Contains(out, "[I]") {
		t.Errorf("expected [I] marker; got:\n%s", out)
	}
	if !strings.Contains(out, "[P]") {
		t.Errorf("expected [P] marker; got:\n%s", out)
	}

	// Per-tier implemented counts
	if !strings.Contains(out, "MUST (deal-breaker) — 1/2 implemented") {
		t.Errorf("expected MUST tier count; got:\n%s", out)
	}
	if !strings.Contains(out, "SHOULD (friction) — 1/1 implemented") {
		t.Errorf("expected SHOULD tier count; got:\n%s", out)
	}
	if !strings.Contains(out, "NICE-TO-HAVE (polish) — 0/1 implemented") {
		t.Errorf("expected NICE-TO-HAVE tier count; got:\n%s", out)
	}
}

func TestPrintFeatureItem_ShipReady(t *testing.T) {
	f := clitypes.FeatureItem{
		Name: "ready-feature",
		Specs: []clitypes.SpecItem{
			{ID: 1, Importance: "must", Description: "fully covered", GreenDemos: 1},
			{ID: 2, Importance: "should", Description: "not yet covered", GreenDemos: 0},
		},
	}

	out := captureStdout(func() { PrintFeatureItem(f) })

	if !strings.Contains(out, "Ship-ready") {
		t.Errorf("expected Ship-ready; got:\n%s", out)
	}
}
