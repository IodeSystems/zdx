package project

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/doctor"
)

// TestPromptClassification verifies spec 130 — promptClassification parses
// numeric choices, name matches (case-insensitive), and falls back to the
// tool default when input is unrecognized.
func TestPromptClassification(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  doctor.Classification
	}{
		{"numeric library", "1\n", doctor.ClassLibrary},
		{"numeric tool", "2\n", doctor.ClassTool},
		{"numeric service", "3\n", doctor.ClassService},
		{"numeric saas", "4\n", doctor.ClassSaaS},
		{"numeric site", "5\n", doctor.ClassSite},
		{"name lowercase", "saas\n", doctor.ClassSaaS},
		{"name mixed case", "SaaS\n", doctor.ClassSaaS},
		{"name with whitespace", "  library  \n", doctor.ClassLibrary},
		{"unknown falls back to tool", "nonsense\n", doctor.ClassTool},
		{"empty falls back to tool", "\n", doctor.ClassTool},
	}

	// silence stdout (the prompt prints menu text)
	pr, pw, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = pw
	t.Cleanup(func() {
		os.Stdout = origStdout
		pw.Close()
		pr.Close()
	})
	go io.Copy(io.Discard, pr)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := promptClassification(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("promptClassification(%q) error: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("promptClassification(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// TestPromptClassificationEOF verifies an empty reader (no newline) returns
// an error rather than silently choosing a default.
func TestPromptClassificationEOF(t *testing.T) {
	pr, pw, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = pw
	t.Cleanup(func() {
		os.Stdout = origStdout
		pw.Close()
		pr.Close()
	})
	go io.Copy(io.Discard, pr)

	if _, err := promptClassification(strings.NewReader("")); err == nil {
		t.Error("expected error from empty reader, got nil")
	}
}

// TestRunDoctorWithFix verifies spec 128 (CLI --fix path):
// runDoctor with autoFix=true calls FixFunc and prints "→ fixed".
func TestRunDoctorWithFix(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// stdin: classification choice, then EOF for any subsequent reads
	sr, sw, _ := os.Pipe()
	_, _ = sw.WriteString("tool\n")
	sw.Close()
	origStdin := os.Stdin
	os.Stdin = sr
	t.Cleanup(func() { os.Stdin = origStdin; sr.Close() })

	// capture stdout
	pr, pw, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = pw

	var buf bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, pr)
		close(copyDone)
	}()

	runErr := runDoctor(context.Background(), true, false)

	pw.Close()
	<-copyDone
	os.Stdout = origStdout

	if runErr != nil {
		t.Fatalf("runDoctor error: %v", runErr)
	}

	out := buf.String()
	if !strings.Contains(out, "→ fixed") {
		t.Errorf("expected '→ fixed' in output; got:\n%s", out)
	}
	if _, statErr := os.Stat(".zdx"); statErr != nil {
		t.Error("expected .zdx/ to be created when --fix is set")
	}
}

// TestRunDoctorWithoutFix verifies spec 128 (CLI no-fix path):
// runDoctor without autoFix prints the auto-fixable hint and does not apply the fix.
func TestRunDoctorWithoutFix(t *testing.T) {
	dir := t.TempDir()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })

	// stdin: classification choice, then EOF for defer prompts
	sr, sw, _ := os.Pipe()
	_, _ = sw.WriteString("tool\n")
	sw.Close()
	origStdin := os.Stdin
	os.Stdin = sr
	t.Cleanup(func() { os.Stdin = origStdin; sr.Close() })

	// capture stdout
	pr, pw, _ := os.Pipe()
	origStdout := os.Stdout
	os.Stdout = pw

	var buf bytes.Buffer
	copyDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, pr)
		close(copyDone)
	}()

	runErr := runDoctor(context.Background(), false, false)

	pw.Close()
	<-copyDone
	os.Stdout = origStdout

	if runErr != nil {
		t.Fatalf("runDoctor error: %v", runErr)
	}

	out := buf.String()
	if !strings.Contains(out, "→ auto-fixable (run with --fix)") {
		t.Errorf("expected auto-fixable hint in output; got:\n%s", out)
	}
	if strings.Contains(out, "→ fixed") {
		t.Errorf("expected no '→ fixed' without --fix; got:\n%s", out)
	}
	if _, statErr := os.Stat(".zdx"); statErr == nil {
		t.Error("expected .zdx/ NOT to be created without --fix")
	}
}
