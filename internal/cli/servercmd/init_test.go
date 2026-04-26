package servercmd_test

import (
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli/servercmd"
)

// setStdin replaces os.Stdin with a pipe pre-loaded with content.
// Returns a restore func the caller must call when done.
func setStdin(content string) func() {
	r, w, _ := os.Pipe()
	old := os.Stdin
	os.Stdin = r
	_, _ = io.WriteString(w, content)
	w.Close()
	return func() {
		os.Stdin = old
		r.Close()
	}
}

// captureOutput redirects os.Stdout and os.Stderr into a pipe, runs fn,
// then returns the collected combined output.
func captureOutput(fn func()) string {
	r, w, _ := os.Pipe()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = w, w
	fn()
	w.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	var buf strings.Builder
	_, _ = io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

// gitInitDir runs git init in dir so scaffoldZdx behaves consistently.
func gitInitDir(t *testing.T, dir string) {
	t.Helper()
	if err := exec.Command("git", "init", dir).Run(); err != nil {
		t.Logf("git init: %v (continuing)", err)
	}
}

// chdirTo changes the working directory to dir and registers a cleanup to
// restore the original directory.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chdir(orig) })
}

func TestInitCmd(t *testing.T) {
	// Subtests run sequentially — os.Chdir and os.Stdin are global state.

	t.Run("fresh_no_container", func(t *testing.T) {
		dir := t.TempDir()
		gitInitDir(t, dir)
		chdirTo(t, dir)

		// Provide classification choice for the doctor's interactive prompt.
		// "2\n" selects ClassTool; subsequent EOF reads are ignored by the
		// defer-prompt loop.
		restoreStdin := setStdin("2\n")
		cmd := servercmd.InitCmd()
		cmd.SetArgs([]string{"--no-container"})
		out := captureOutput(func() { _ = cmd.Execute() })
		restoreStdin()

		// .zdx/config.yaml exists and contains both skeleton-block markers.
		data, err := os.ReadFile(".zdx/config.yaml")
		if err != nil {
			t.Fatalf(".zdx/config.yaml: %v", err)
		}
		for _, marker := range []string{"hooks:", "components:"} {
			if !strings.Contains(string(data), marker) {
				t.Errorf("config.yaml missing %q block", marker)
			}
		}

		// .gitignore has all six expected entries.
		gitignore, err := os.ReadFile(".gitignore")
		if err != nil {
			t.Fatalf(".gitignore: %v", err)
		}
		for _, entry := range []string{
			".zdx/context",
			".zdx/credentials",
			".zdx/test-results.json",
			".zdx/test-index.json",
			".zdx/smoke.json",
			".zdx/metrics-latest.yaml",
		} {
			if !strings.Contains(string(gitignore), entry) {
				t.Errorf(".gitignore missing entry %q", entry)
			}
		}

		// Dev container files are absent with --no-container.
		for _, f := range []string{"dev.Dockerfile", "docker-compose.dev.yml"} {
			if _, statErr := os.Stat(f); statErr == nil {
				t.Errorf("unexpected file %s (should be absent with --no-container)", f)
			}
		}

		// Doctor ran (without --fix).
		if !strings.Contains(out, "running project health check") {
			t.Errorf("expected doctor output; combined output:\n%s", out)
		}
	})

	t.Run("fresh_with_container", func(t *testing.T) {
		dir := t.TempDir()
		gitInitDir(t, dir)
		chdirTo(t, dir)

		restoreStdin := setStdin("2\n")
		cmd := servercmd.InitCmd()
		cmd.SetArgs([]string{"--with-container"})
		out := captureOutput(func() { _ = cmd.Execute() })
		restoreStdin()

		// Dev container files must exist after --with-container.
		for _, f := range []string{"dev.Dockerfile", "docker-compose.dev.yml"} {
			if _, statErr := os.Stat(f); statErr != nil {
				t.Errorf("expected %s after --with-container: %v", f, statErr)
			}
		}

		// Doctor ran (with --fix).
		if !strings.Contains(out, "running project health check") {
			t.Errorf("expected doctor output; combined output:\n%s", out)
		}
	})

	t.Run("rerun_no_duplicate_entries", func(t *testing.T) {
		dir := t.TempDir()
		gitInitDir(t, dir)
		chdirTo(t, dir)

		// First run.
		restore1 := setStdin("2\n")
		cmd1 := servercmd.InitCmd()
		cmd1.SetArgs([]string{"--no-container"})
		captureOutput(func() { _ = cmd1.Execute() })
		restore1()

		original, err := os.ReadFile(".zdx/config.yaml")
		if err != nil {
			t.Fatalf("config after first run: %v", err)
		}

		// Second run.
		restore2 := setStdin("2\n")
		cmd2 := servercmd.InitCmd()
		cmd2.SetArgs([]string{"--no-container"})
		out := captureOutput(func() { _ = cmd2.Execute() })
		restore2()

		if !strings.Contains(out, "already exists") {
			t.Errorf("expected 'already exists' on re-run; output:\n%s", out)
		}

		// Config content unchanged.
		after, _ := os.ReadFile(".zdx/config.yaml")
		if string(original) != string(after) {
			t.Error(".zdx/config.yaml content changed on re-run")
		}

		// No duplicate .gitignore entries.
		gitignore, _ := os.ReadFile(".gitignore")
		if n := strings.Count(string(gitignore), ".zdx/context"); n > 1 {
			t.Errorf(".gitignore entry '.zdx/context' duplicated %d times", n)
		}
	})

	t.Run("doctor_identity_proposals", func(t *testing.T) {
		dir := t.TempDir()
		gitInitDir(t, dir)
		chdirTo(t, dir)

		// Without server credentials, populateRemoteState is skipped so all
		// identity checks (vision, goals, constraints) default to zero/unset
		// and produce ActionPropose output.
		restoreStdin := setStdin("2\n")
		cmd := servercmd.InitCmd()
		cmd.SetArgs([]string{"--no-container"})
		out := captureOutput(func() { _ = cmd.Execute() })
		restoreStdin()

		for _, proposal := range []string{"dx vision set", "dx goal add"} {
			if !strings.Contains(out, proposal) {
				t.Errorf("expected identity proposal %q in doctor output:\n%s", proposal, out)
			}
		}
	})
}
