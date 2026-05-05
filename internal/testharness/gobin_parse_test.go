package testharness

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestParseGoTestJSON_LargeOutputLine reproduces IS-1030: test2json emits an
// "output" event per line of subprocess output. When a single line is huge
// (e.g. TestDemoCLI_* spawns vitest which dumps verbose UI test logs), the
// resulting JSON line can blow past Scanner's default 64KB buffer. Pre-fix,
// Scanner errored silently, all subsequent events dropped, results came back
// empty even though tests genuinely passed.
func TestParseGoTestJSON_LargeOutputLine(t *testing.T) {
	huge := strings.Repeat("x", 200*1024) // 200KB — well above default 64KB
	hugeOut, _ := json.Marshal(huge)      // produces JSON string with 200KB+ of payload

	// Mix in: a tiny output, then the huge output, then the terminal pass.
	// All for the same test name. The bug dropped the pass event because
	// the huge output line broke the scanner.
	stream := strings.Join([]string{
		`{"Action":"run","Test":"TestBig"}`,
		`{"Action":"output","Test":"TestBig","Output":"starting\n"}`,
		`{"Action":"output","Test":"TestBig","Output":` + string(hugeOut) + `}`,
		`{"Action":"output","Test":"TestBig","Output":"--- PASS: TestBig (0.42s)\n"}`,
		`{"Action":"pass","Test":"TestBig","Elapsed":0.42}`,
	}, "\n")

	results := parseGoTestJSON([]byte(stream), "api", LayerIntegration)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Test != "TestBig" {
		t.Errorf("Test = %q, want TestBig", results[0].Test)
	}
	if results[0].Status != "pass" {
		t.Errorf("Status = %q, want pass", results[0].Status)
	}
	if results[0].DurationMs != 420 {
		t.Errorf("DurationMs = %d, want 420", results[0].DurationMs)
	}
}

// Also verify the common case (small output, multiple tests) still works
// after the buffer change — a sanity check that we didn't regress parsing.
func TestParseGoTestJSON_MultipleSmallTests(t *testing.T) {
	stream := strings.Join([]string{
		`{"Action":"run","Test":"TestA"}`,
		`{"Action":"output","Test":"TestA","Output":"--- PASS: TestA (0.01s)\n"}`,
		`{"Action":"pass","Test":"TestA","Elapsed":0.01}`,
		`{"Action":"run","Test":"TestB"}`,
		`{"Action":"output","Test":"TestB","Output":"--- FAIL: TestB (0.05s)\n"}`,
		`{"Action":"fail","Test":"TestB","Elapsed":0.05}`,
		`{"Action":"run","Test":"TestC"}`,
		`{"Action":"output","Test":"TestC","Output":"--- SKIP: TestC (0.00s)\n"}`,
		`{"Action":"skip","Test":"TestC","Elapsed":0.0}`,
	}, "\n")

	results := parseGoTestJSON([]byte(stream), "api", LayerUnit)
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	wantStatus := map[string]string{"TestA": "pass", "TestB": "fail", "TestC": "skip"}
	for _, r := range results {
		if got, ok := wantStatus[r.Test]; !ok || got != r.Status {
			t.Errorf("Test=%q Status=%q, want %q", r.Test, r.Status, got)
		}
	}
}
