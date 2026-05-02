package testharness

import (
	"strings"
	"testing"
)

func TestGobinRunArgs_ForwardsNameFilter(t *testing.T) {
	args := gobinRunArgs(Filter{Name: "TestFoo"})
	if !containsArg(args, "-test.run=TestFoo") {
		t.Errorf("expected -test.run=TestFoo in %v", args)
	}
}

func TestGobinRunArgs_ForwardsFeatureFilter(t *testing.T) {
	args := gobinRunArgs(Filter{Feature: "auth"})
	if !containsArg(args, "-test.run=auth") {
		t.Errorf("expected -test.run=auth in %v", args)
	}
}

func TestGobinRunArgs_NameTakesPrecedenceOverFeature(t *testing.T) {
	args := gobinRunArgs(Filter{Name: "TestFoo", Feature: "auth"})
	if !containsArg(args, "-test.run=TestFoo") {
		t.Errorf("expected -test.run=TestFoo in %v", args)
	}
	for _, a := range args {
		if strings.Contains(a, "auth") {
			t.Errorf("feature should not appear when Name is set, got %v", args)
		}
	}
}

func TestGobinRunArgs_EmptyFilter_NoRunFlag(t *testing.T) {
	args := gobinRunArgs(Filter{})
	for _, a := range args {
		if strings.HasPrefix(a, "-test.run") {
			t.Errorf("empty filter should not add -test.run, got %v", args)
		}
	}
}

func TestVitestRunArgs_ForwardsNameFilter(t *testing.T) {
	args := jsRunArgs("vitest", Filter{Name: "Foo"}, "/tmp/out.json")
	if !containsArg(args, "--testNamePattern=Foo") {
		t.Errorf("expected --testNamePattern=Foo in %v", args)
	}
}

func TestVitestRunArgs_ForwardsFeatureFilter(t *testing.T) {
	args := jsRunArgs("vitest", Filter{Feature: "login"}, "/tmp/out.json")
	if !containsArg(args, "--testNamePattern=login") {
		t.Errorf("expected --testNamePattern=login in %v", args)
	}
}

func TestVitestRunArgs_EmptyFilter_NoPatternFlag(t *testing.T) {
	args := jsRunArgs("vitest", Filter{}, "/tmp/out.json")
	for _, a := range args {
		if strings.HasPrefix(a, "--testNamePattern") {
			t.Errorf("empty filter should not add --testNamePattern, got %v", args)
		}
	}
}

func TestVitestRunArgs_OutputFileIncluded(t *testing.T) {
	args := jsRunArgs("vitest", Filter{}, "/my/output.json")
	if !containsArg(args, "--outputFile=/my/output.json") {
		t.Errorf("expected --outputFile=/my/output.json in %v", args)
	}
}

func TestJestRunArgs_ForwardsNameFilter(t *testing.T) {
	args := jsRunArgs("jest", Filter{Name: "Foo"}, "/tmp/out.json")
	if !containsArg(args, "--testNamePattern=Foo") {
		t.Errorf("expected --testNamePattern=Foo in %v", args)
	}
}

func TestJestRunArgs_OutputFileIncluded(t *testing.T) {
	args := jsRunArgs("jest", Filter{}, "/my/output.json")
	if !containsArg(args, "--outputFile=/my/output.json") {
		t.Errorf("expected --outputFile=/my/output.json in %v", args)
	}
	if !containsArg(args, "--json") {
		t.Errorf("expected --json in %v", args)
	}
}

func containsArg(args []string, want string) bool {
	for _, a := range args {
		if a == want {
			return true
		}
	}
	return false
}
