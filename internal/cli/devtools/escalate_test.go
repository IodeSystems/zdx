package devtools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/testharness"
)

func TestBuildFingerprint(t *testing.T) {
	r := testharness.Result{
		Test:      "TestFoo",
		Component: "api",
		Output:    "=== RUN   TestFoo\n--- FAIL: TestFoo\nerror: connection refused\nstack trace...",
	}
	fp := buildFingerprint(r)
	if fp == "" {
		t.Fatal("fingerprint should not be empty")
	}
	if fp != "test failure: TestFoo runner=api error=error: connection refused" {
		t.Errorf("unexpected fingerprint: %q", fp)
	}
}

func TestBuildFingerprint_NoOutput(t *testing.T) {
	r := testharness.Result{
		Test:      "TestBar",
		Component: "ui",
		Output:    "",
	}
	fp := buildFingerprint(r)
	expected := "test failure: TestBar runner=ui error=(no error output)"
	if fp != expected {
		t.Errorf("unexpected fingerprint: %q, want %q", fp, expected)
	}
}

func TestBuildFingerprint_OnlyVerboseLines(t *testing.T) {
	r := testharness.Result{
		Test:      "TestBaz",
		Component: "demo",
		Output:    "=== RUN   TestBaz\n=== PAUSE TestBaz\n=== CONT  TestBaz\n--- FAIL: TestBaz",
	}
	fp := buildFingerprint(r)
	expected := "test failure: TestBaz runner=demo error=(no error output)"
	if fp != expected {
		t.Errorf("unexpected fingerprint: %q, want %q", fp, expected)
	}
}

func TestFirstErrorLine(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   string
	}{
		{
			name:   "simple error",
			output: "=== RUN Test\nerror: something broke\nstack...",
			want:   "error: something broke",
		},
		{
			name:   "verbose prefix",
			output: "=== RUN Test\n--- FAIL: Test\nwant ok, got err\nstack...",
			want:   "want ok, got err",
		},
		{
			name:   "empty output",
			output: "",
			want:   "(no error output)",
		},
		{
			name:   "only verbose lines",
			output: "=== RUN Test\n=== PAUSE Test\n=== CONT Test\n--- FAIL: Test",
			want:   "(no error output)",
		},
		{
			name:   "long line skipped",
			output: "=== RUN Test\n" + strings.Repeat("x", 201) + "\nshort error\n",
			want:   "short error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := firstErrorLine(tt.output)
			if got != tt.want {
				t.Errorf("firstErrorLine() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	short := "hello"
	if got := truncate(short, 10); got != short {
		t.Errorf("truncate(short, 10) = %q, want %q", got, short)
	}
	long := strings.Repeat("x", 100)
	got := truncate(long, 10)
	if len(got) != 13 { // 10 + "..."
		t.Errorf("truncate(long, 10) len = %d, want 13", len(got))
	}
	if got != "xxxxxxxxxx..." {
		t.Errorf("truncate(long, 10) = %q, want \"xxxxxxxxxx...\"", got)
	}
}

func TestWriteEscalationJSON(t *testing.T) {
	results := []EscalationResult{
		{TestName: "TestFoo", Action: "filed", IssueID: "IS-100", Fingerprint: "fp1"},
		{TestName: "TestBar", Action: "recurrence", IssueID: "IS-99", Fingerprint: "fp2"},
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "escalation.json")
	if err := WriteEscalationJSON(path, results); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var got []EscalationResult
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Action != "filed" {
		t.Errorf("first action = %q, want %q", got[0].Action, "filed")
	}
	if got[1].Action != "recurrence" {
		t.Errorf("second action = %q, want %q", got[1].Action, "recurrence")
	}
}

func TestWriteEscalationJSON_Empty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "escalation.json")
	err := WriteEscalationJSON(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected no file for empty results")
	}
}

func TestPrintEscalationSummary(t *testing.T) {
	// Just verify it doesn't panic.
	results := []EscalationResult{
		{TestName: "TestFoo", Action: "filed", IssueID: "IS-100"},
		{TestName: "TestBar", Action: "recurrence", IssueID: "IS-99"},
		{TestName: "TestBaz", Action: "failed"},
	}
	PrintEscalationSummary(results)
	PrintEscalationSummary(nil)
}

func TestFilterFailures(t *testing.T) {
	results := []testharness.Result{
		{Test: "TestPass", Status: "pass"},
		{Test: "TestFail1", Status: "fail"},
		{Test: "TestSkip", Status: "skip"},
		{Test: "TestFail2", Status: "fail"},
	}
	failures := filterFailures(results)
	if len(failures) != 2 {
		t.Fatalf("expected 2 failures, got %d", len(failures))
	}
	if failures[0].Test != "TestFail1" || failures[1].Test != "TestFail2" {
		t.Errorf("unexpected failures: %v", failures)
	}
}

func TestFilterFailures_None(t *testing.T) {
	results := []testharness.Result{
		{Test: "TestPass", Status: "pass"},
		{Test: "TestSkip", Status: "skip"},
	}
	failures := filterFailures(results)
	if len(failures) != 0 {
		t.Fatalf("expected 0 failures, got %d", len(failures))
	}
}
