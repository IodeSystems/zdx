package e2e

import (
	"strings"
	"testing"
)

// TestDemoCLI_AddHelpGuidance is the demo for spec 110: when any entity-add
// command is invoked with --help, the Long description explains what makes a
// good instance of that entity type and includes a concrete example.
//
// Covers goal, feature, spec, and issue — the four SDLC hierarchy entities
// that have add commands with Long guidance.
func TestDemoCLI_AddHelpGuidance(t *testing.T) {
	rec := newRecorder(t, "add-help-guidance", "bin/dx")
	t.Cleanup(rec.Save)

	// goal add --help: must explain measurable outcomes and include a metric example.
	rec.Run("goal", "add", "--help")
	goalHelp := rec.steps[len(rec.steps)-1].Stdout
	for _, want := range []string{"good", "example", "metric"} {
		if !strings.Contains(strings.ToLower(goalHelp), want) {
			t.Errorf("goal add --help: missing %q\n%s", want, goalHelp)
		}
	}

	// feature add --help: must explain direct vs multiplier kinds with examples.
	rec.Run("feature", "add", "--help")
	featureHelp := rec.steps[len(rec.steps)-1].Stdout
	for _, want := range []string{"good", "example", "direct", "multiplier"} {
		if !strings.Contains(strings.ToLower(featureHelp), want) {
			t.Errorf("feature add --help: missing %q\n%s", want, featureHelp)
		}
	}

	// spec add --help: must explain must/should/nice-to-have kinds with examples.
	rec.Run("spec", "add", "--help")
	specHelp := rec.steps[len(rec.steps)-1].Stdout
	for _, want := range []string{"good", "example", "must", "should", "nice-to-have"} {
		if !strings.Contains(strings.ToLower(specHelp), want) {
			t.Errorf("spec add --help: missing %q\n%s", want, specHelp)
		}
	}

	// issue add --help: must explain impl/ops/ask types with examples.
	rec.Run("issue", "add", "--help")
	issueHelp := rec.steps[len(rec.steps)-1].Stdout
	for _, want := range []string{"good", "example", "impl", "ops", "ask"} {
		if !strings.Contains(strings.ToLower(issueHelp), want) {
			t.Errorf("issue add --help: missing %q\n%s", want, issueHelp)
		}
	}

	for _, s := range rec.steps {
		if s.ExitCode != 0 {
			t.Errorf("step %q exited %d:\n%s", s.Cmd, s.ExitCode, s.Stderr)
		}
	}
}
