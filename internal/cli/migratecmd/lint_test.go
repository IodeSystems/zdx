package migratecmd

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestLint_Clean(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 5; i++ {
		writePair(t, dir, i, "step")
	}
	defects := LintMigrations(dir, "")
	if len(defects) != 0 {
		t.Errorf("expected no defects; got %v", defects)
	}
}

func TestLint_Duplicate(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "alpha")
	writePair(t, dir, 1, "bravo")
	defects := LintMigrations(dir, "")
	if !hasKind(defects, "duplicate") {
		t.Errorf("expected duplicate defect; got %v", defects)
	}
}

func TestLint_MissingDown(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "step")
	writeHalf(t, dir, 2, "orphan", "up")
	defects := LintMigrations(dir, "")
	if !hasKind(defects, "missing-pair") {
		t.Errorf("expected missing-pair defect; got %v", defects)
	}
	if !containsMsg(defects, "002_orphan.up.sql") {
		t.Errorf("expected message about 002_orphan.up.sql; got %v", defects)
	}
}

func TestLint_MissingUp(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "step")
	writeHalf(t, dir, 2, "orphan", "down")
	defects := LintMigrations(dir, "")
	if !hasKind(defects, "missing-pair") {
		t.Errorf("expected missing-pair defect; got %v", defects)
	}
	if !containsMsg(defects, "002_orphan.down.sql") {
		t.Errorf("expected message about 002_orphan.down.sql; got %v", defects)
	}
}

func TestLint_Gap(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 81, "a")
	writePair(t, dir, 83, "b")
	defects := LintMigrations(dir, "")
	if !hasKind(defects, "gap") {
		t.Errorf("expected gap defect; got %v", defects)
	}
	if !containsMsg(defects, "081") || !containsMsg(defects, "083") {
		t.Errorf("expected gap message referencing 081 and 083; got %v", defects)
	}
}

func TestLint_RealMigrations(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	// thisFile is .../internal/cli/migratecmd/lint_test.go; go up 3 dirs to repo root.
	repoRoot := filepath.Join(filepath.Dir(thisFile), "..", "..", "..")
	dir := filepath.Join(repoRoot, defaultMigrationsDir)
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("real migrations dir not found: %v", err)
	}
	defects := LintMigrations(dir, "")
	for _, d := range defects {
		t.Errorf("real migrations dir has defect: %s: %s", d.Kind, d.Message)
	}
}

func TestLint_StaleSchema_Missing(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "step")
	schemaFile := filepath.Join(t.TempDir(), "shipped.sql")
	// schemaFile does not exist
	defects := LintMigrations(dir, schemaFile)
	if !hasKind(defects, "stale-schema") {
		t.Errorf("expected stale-schema defect when shipped.sql missing; got %v", defects)
	}
}

func TestLint_StaleSchema_Stale(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "step")

	schemaFile := filepath.Join(t.TempDir(), "shipped.sql")
	if err := os.WriteFile(schemaFile, []byte("-- old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// Touch migration file to be newer than schemaFile.
	future := time.Now().Add(2 * time.Second)
	migFile := filepath.Join(dir, "001_step.up.sql")
	if err := os.Chtimes(migFile, future, future); err != nil {
		t.Fatal(err)
	}

	defects := LintMigrations(dir, schemaFile)
	if !hasKind(defects, "stale-schema") {
		t.Errorf("expected stale-schema defect when migration newer than shipped.sql; got %v", defects)
	}
}

func TestLint_StaleSchema_Fresh(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "step")

	schemaFile := filepath.Join(t.TempDir(), "shipped.sql")
	// Write schema file after migrations so it is newer.
	if err := os.WriteFile(schemaFile, []byte("-- current\n"), 0644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(schemaFile, future, future); err != nil {
		t.Fatal(err)
	}

	defects := LintMigrations(dir, schemaFile)
	if hasKind(defects, "stale-schema") {
		t.Errorf("expected no stale-schema defect when shipped.sql is fresh; got %v", defects)
	}
}

func writeHalf(t *testing.T, dir string, nnn int, name, side string) {
	t.Helper()
	path := filepath.Join(dir, formatN(nnn)+"_"+name+"."+side+".sql")
	if err := os.WriteFile(path, []byte("-- "+side+"\n"), 0644); err != nil {
		t.Fatalf("writeHalf %s: %v", path, err)
	}
}

func hasKind(defects []Defect, kind string) bool {
	for _, d := range defects {
		if d.Kind == kind {
			return true
		}
	}
	return false
}

func containsMsg(defects []Defect, sub string) bool {
	for _, d := range defects {
		if strings.Contains(d.Message, sub) {
			return true
		}
	}
	return false
}
