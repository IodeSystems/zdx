package ship

import (
	"path/filepath"
	"testing"
)

func TestStateFilePath(t *testing.T) {
	got := stateFilePath("", "abc123", "mycomp", "current")
	want := filepath.Join(".zdx", "ship-state", "abc123-mycomp-current.json")
	if got != want {
		t.Errorf("stateFilePath = %q, want %q", got, want)
	}
}

func TestStateFilePath_CustomDir(t *testing.T) {
	got := stateFilePath("/tmp/mydir", "abc123", "mycomp", "next")
	want := "/tmp/mydir/abc123-mycomp-next.json"
	if got != want {
		t.Errorf("stateFilePath = %q, want %q", got, want)
	}
}

func TestLoadCompletedStages_Missing(t *testing.T) {
	dir := t.TempDir()
	got, err := loadCompletedStages(filepath.Join(dir, "nonexistent.json"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %v", got)
	}
}

func TestAppendCompletedStage_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := appendCompletedStage(path, "build"); err != nil {
		t.Fatalf("append build: %v", err)
	}
	if err := appendCompletedStage(path, "rsync"); err != nil {
		t.Fatalf("append rsync: %v", err)
	}

	got, err := loadCompletedStages(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got["build"] || !got["rsync"] {
		t.Errorf("loaded set = %v, want build and rsync", got)
	}
	if len(got) != 2 {
		t.Errorf("loaded set len = %d, want 2", len(got))
	}
}

func TestAppendCompletedStage_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	for i := 0; i < 3; i++ {
		if err := appendCompletedStage(path, "build"); err != nil {
			t.Fatalf("append iteration %d: %v", i, err)
		}
	}

	got, err := loadCompletedStages(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("want 1 entry after idempotent appends, got %d: %v", len(got), got)
	}
}

func TestAppendCompletedStage_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "dir", "state.json")

	if err := appendCompletedStage(path, "deploy"); err != nil {
		t.Fatalf("append: %v", err)
	}
	got, err := loadCompletedStages(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got["deploy"] {
		t.Errorf("want deploy in loaded set, got %v", got)
	}
}
