package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMissingColumns_Single(t *testing.T) {
	stderr := `# package
queries/issues.sql:5:1: column "completed_in_sha" of relation "zdx_issues" does not exist`
	got := parseMissingColumns(stderr)
	if len(got) != 1 {
		t.Fatalf("want 1 match, got %d: %+v", len(got), got)
	}
	if got[0].Column != "completed_in_sha" || got[0].Table != "zdx_issues" {
		t.Errorf("got %+v", got[0])
	}
}

func TestParseMissingColumns_BareForm(t *testing.T) {
	stderr := `column "timeout_seconds" does not exist`
	got := parseMissingColumns(stderr)
	if len(got) != 1 || got[0].Column != "timeout_seconds" || got[0].Table != "" {
		t.Fatalf("got %+v", got)
	}
}

func TestParseMissingColumns_PrefersTableQualified(t *testing.T) {
	stderr := `column "x" of relation "t" does not exist
column "x" does not exist`
	got := parseMissingColumns(stderr)
	if len(got) != 1 {
		t.Fatalf("want 1 dedup match, got %d: %+v", len(got), got)
	}
	if got[0].Table != "t" {
		t.Errorf("want table=t, got %+v", got[0])
	}
}

func TestParseMissingColumns_MultipleUnique(t *testing.T) {
	stderr := `column "a" of relation "t1" does not exist
column "b" of relation "t2" does not exist
column "a" of relation "t1" does not exist`
	got := parseMissingColumns(stderr)
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(got), got)
	}
}

func TestParseMissingColumns_None(t *testing.T) {
	stderr := `some unrelated error`
	if got := parseMissingColumns(stderr); len(got) != 0 {
		t.Fatalf("want 0, got %+v", got)
	}
}

func TestFindAddingMigration_HighestNumber(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "010_add_foo.up.sql"),
		`ALTER TABLE zdx_issues ADD COLUMN foo TEXT;`)
	mustWrite(t, filepath.Join(dir, "020_add_foo_again.up.sql"),
		`ALTER TABLE zdx_other ADD COLUMN foo TEXT;`)

	got := findAddingMigration(dir, "foo", "zdx_other")
	if got != "020_add_foo_again.up.sql" {
		t.Errorf("want highest-numbered match for the right table, got %q", got)
	}

	// without table, highest-numbered any match wins
	got = findAddingMigration(dir, "foo", "")
	if got != "020_add_foo_again.up.sql" {
		t.Errorf("want 020 (highest), got %q", got)
	}
}

func TestFindAddingMigration_IfNotExists(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "030_vision.up.sql"),
		`ALTER TABLE zdx_projects ADD COLUMN IF NOT EXISTS title text NOT NULL DEFAULT '';`)
	got := findAddingMigration(dir, "title", "zdx_projects")
	if got != "030_vision.up.sql" {
		t.Errorf("want 030, got %q", got)
	}
}

func TestFindAddingMigration_QuotedIdent(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "040_quoted.up.sql"),
		`ALTER TABLE "zdx_things" ADD COLUMN "quoted_col" INT;`)
	got := findAddingMigration(dir, "quoted_col", "zdx_things")
	if got != "040_quoted.up.sql" {
		t.Errorf("want 040, got %q", got)
	}
}

func TestFindAddingMigration_RejectsCrossStatement(t *testing.T) {
	dir := t.TempDir()
	// foo is added on table_b, NOT table_a. The file also touches table_a
	// for an unrelated column. The matcher must not credit this file when
	// asked about (foo, table_a).
	mustWrite(t, filepath.Join(dir, "050_mixed.up.sql"),
		`ALTER TABLE table_a ADD COLUMN bar INT;
ALTER TABLE table_b ADD COLUMN foo INT;`)
	got := findAddingMigration(dir, "foo", "table_a")
	if got != "" {
		t.Errorf("want no match (wrong table), got %q", got)
	}
	got = findAddingMigration(dir, "foo", "table_b")
	if got != "050_mixed.up.sql" {
		t.Errorf("want 050, got %q", got)
	}
}

func TestFindAddingMigration_NoMatch(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "060_other.up.sql"),
		`ALTER TABLE foo ADD COLUMN bar INT;`)
	if got := findAddingMigration(dir, "missing_col", ""); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

func TestFindAddingMigration_IgnoresDownFiles(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "070_thing.down.sql"),
		`ALTER TABLE t ADD COLUMN c INT;`)
	if got := findAddingMigration(dir, "c", "t"); got != "" {
		t.Errorf("down.sql files must not match, got %q", got)
	}
}

func TestFormatHint_NamesMigration(t *testing.T) {
	cols := []missingColumn{{Column: "completed_in_sha", Table: "zdx_issues"}}
	out := formatHint("internal/migrate/sql", cols, func(col, table string) string {
		return "153_issue_completed_sha.up.sql"
	})
	if !strings.Contains(out, "completed_in_sha") {
		t.Errorf("hint missing column name: %q", out)
	}
	if !strings.Contains(out, "153_issue_completed_sha.up.sql") {
		t.Errorf("hint missing migration filename: %q", out)
	}
	if !strings.Contains(out, "bin/regen-schema") {
		t.Errorf("hint missing regen pointer: %q", out)
	}
}

func TestFormatHint_FallbackWhenUnknown(t *testing.T) {
	cols := []missingColumn{{Column: "ghost", Table: "zdx_ghosts"}}
	out := formatHint("internal/migrate/sql", cols, func(col, table string) string { return "" })
	if !strings.Contains(out, "no ADD COLUMN found") {
		t.Errorf("want fallback message, got %q", out)
	}
}

func TestFormatHint_EmptyReturnsEmpty(t *testing.T) {
	if formatHint("x", nil, nil) != "" {
		t.Errorf("want empty for empty cols")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
