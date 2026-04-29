package project

import (
	"bytes"
	"strings"
	"testing"
)

// spec 110: each entity-add command must emit Long help with guidance and an example.
func TestGoalAddHelpGuidance(t *testing.T) {
	cmd := GoalCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"add", "--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, want := range []string{"good", "example", "metric"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("goal add --help: missing %q in output:\n%s", want, out)
		}
	}
}

func TestFeatureAddHelpGuidance(t *testing.T) {
	cmd := FeatureCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"add", "--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, want := range []string{"good", "example", "direct", "multiplier"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("feature add --help: missing %q in output:\n%s", want, out)
		}
	}
}

func TestSpecAddHelpGuidance(t *testing.T) {
	cmd := SpecCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"add", "--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, want := range []string{"good", "example", "must", "should", "nice-to-have"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("spec add --help: missing %q in output:\n%s", want, out)
		}
	}
}

func TestIssueAddHelpGuidance(t *testing.T) {
	cmd := IssueCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetErr(&buf)
	cmd.SetArgs([]string{"add", "--help"})
	_ = cmd.Execute()
	out := buf.String()
	for _, want := range []string{"good", "example", "impl", "ops", "ask"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("issue add --help: missing %q in output:\n%s", want, out)
		}
	}
}
