package testharness

import (
	"os"
	"testing"
)

const vitestFixture = `{
  "testResults": [
    {
      "name": "/app/ui/src/foo.test.ts",
      "status": "failed",
      "assertionResults": [
        {
          "fullName": "foo passes correctly",
          "status": "passed",
          "duration": 12.5,
          "failureMessages": []
        },
        {
          "fullName": "foo fails on bad input",
          "status": "failed",
          "duration": 3.0,
          "failureMessages": ["Expected true, got false", "second line"]
        }
      ]
    },
    {
      "name": "/app/ui/src/bar.test.ts",
      "status": "passed",
      "assertionResults": [
        {
          "fullName": "bar skips when disabled",
          "status": "skipped",
          "duration": 0.0,
          "failureMessages": []
        }
      ]
    }
  ]
}`

func TestParseVitestReport_MapsResults(t *testing.T) {
	f, err := os.CreateTemp("", "vitest-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(vitestFixture); err != nil {
		t.Fatal(err)
	}
	f.Close()

	results, err := parseVitestReport(f.Name(), "ui")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	pass := results[0]
	if pass.Test != "foo passes correctly" {
		t.Errorf("Test = %q", pass.Test)
	}
	if pass.Status != "pass" {
		t.Errorf("Status = %q, want pass", pass.Status)
	}
	if pass.Component != "ui" {
		t.Errorf("Component = %q, want ui", pass.Component)
	}
	if pass.Layer != LayerUnit {
		t.Errorf("Layer = %q, want %q", pass.Layer, LayerUnit)
	}
	if pass.DurationMs != 12 {
		t.Errorf("DurationMs = %d, want 12", pass.DurationMs)
	}
	if pass.Output != "" {
		t.Errorf("Output = %q, want empty", pass.Output)
	}

	fail := results[1]
	if fail.Status != "fail" {
		t.Errorf("Status = %q, want fail", fail.Status)
	}
	if fail.Output != "Expected true, got false\nsecond line\n" {
		t.Errorf("Output = %q", fail.Output)
	}

	skip := results[2]
	if skip.Status != "skip" {
		t.Errorf("Status = %q, want skip", skip.Status)
	}
}

func TestParseVitestReport_UnreadablePath(t *testing.T) {
	_, err := parseVitestReport("/nonexistent/path/vitest.json", "ui")
	if err == nil {
		t.Fatal("expected error for unreadable path")
	}
}

func TestParseVitestReport_MalformedJSON(t *testing.T) {
	f, err := os.CreateTemp("", "vitest-bad-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.WriteString(`{not valid json`)
	f.Close()

	_, err = parseVitestReport(f.Name(), "ui")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}
