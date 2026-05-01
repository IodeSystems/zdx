package preflight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture sets up an isolated git repo with the given files, commits them,
// and returns the repo root and the resulting commit SHA. After the commit,
// the working tree is left untouched; tests mutate it as the case demands.
type fixture struct {
	root string
	sha  string
	dir  string // migration directory relative path
}

func newFixture(t *testing.T, files map[string]string) *fixture {
	t.Helper()
	root := t.TempDir()
	migDir := "internal/migrate/sql"

	gitRun(t, root, "init", "-q")
	gitRun(t, root, "config", "user.email", "test@example.com")
	gitRun(t, root, "config", "user.name", "Test")
	gitRun(t, root, "config", "commit.gpgsign", "false")

	for rel, body := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	gitRun(t, root, "add", ".")
	gitRun(t, root, "commit", "-q", "-m", "fixture")

	out := gitOutput(t, root, "rev-parse", "HEAD")
	return &fixture{root: root, sha: strings.TrimSpace(out), dir: migDir}
}

func (f *fixture) opts() Options {
	return Options{
		DeployedSHA:  f.sha,
		MigrationDir: f.dir,
		FixesDir:     filepath.Join(f.dir, "migration-fixes"),
	}
}

func (f *fixture) write(t *testing.T, rel, body string) {
	t.Helper()
	full := filepath.Join(f.root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func (f *fixture) remove(t *testing.T, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(f.root, rel)); err != nil {
		t.Fatalf("remove: %v", err)
	}
}

func (f *fixture) run(t *testing.T) ([]Note, []PendingFix, error) {
	t.Helper()
	cwd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(cwd) })
	if err := os.Chdir(f.root); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	return MigrationHistory(context.Background(), f.opts())
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %s: %v", strings.Join(args, " "), err)
	}
	return string(out)
}

func sha8(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])[:8]
}

func TestMigrationHistory_FirstDeploy(t *testing.T) {
	notes, fixes, err := MigrationHistory(context.Background(), Options{DeployedSHA: ""})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fixes) != 0 {
		t.Fatalf("fixes: %v", fixes)
	}
	if len(notes) != 1 || notes[0].Kind != NoteFirstDeploy {
		t.Fatalf("notes: %+v", notes)
	}
}

func TestMigrationHistory_DirtyStripped(t *testing.T) {
	notes, _, err := MigrationHistory(context.Background(), Options{DeployedSHA: "-dirty"})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(notes) != 1 || notes[0].Kind != NoteFirstDeploy {
		t.Fatalf("notes: %+v", notes)
	}
}

func TestMigrationHistory_Clean(t *testing.T) {
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": "create table a();",
	})
	notes, fixes, err := f.run(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(notes) != 0 {
		t.Fatalf("expected no notes, got %+v", notes)
	}
	if len(fixes) != 0 {
		t.Fatalf("expected no fixes, got %+v", fixes)
	}
}

func TestMigrationHistory_PureRename(t *testing.T) {
	body := "create table a();"
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": body,
	})
	// Rename locally — same content, new name.
	f.remove(t, "internal/migrate/sql/001_init.up.sql")
	f.write(t, "internal/migrate/sql/001_init_v2.up.sql", body)

	notes, fixes, err := f.run(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(fixes) != 0 {
		t.Fatalf("fixes: %v", fixes)
	}
	if len(notes) != 1 || notes[0].Kind != NoteRenamed {
		t.Fatalf("notes: %+v", notes)
	}
	if !strings.Contains(notes[0].Msg, "001_init.up.sql → 001_init_v2.up.sql") {
		t.Fatalf("msg: %s", notes[0].Msg)
	}
}

func TestMigrationHistory_RenameModifiedWithFix(t *testing.T) {
	body := "create table a();"
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": body,
	})
	f.remove(t, "internal/migrate/sql/001_init.up.sql")
	f.write(t, "internal/migrate/sql/001_init_v2.up.sql", "create table a (id int);") // different content
	server8 := sha8(body)
	fixName := "001_init.up-" + server8 + "-deadbeef.sql"
	f.write(t, "internal/migrate/sql/migration-fixes/"+fixName, "-- fix")

	notes, fixes, err := f.run(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(notes) != 1 || notes[0].Kind != NoteRenamedModified {
		t.Fatalf("notes: %+v", notes)
	}
	if len(fixes) != 1 || filepath.Base(fixes[0].Path) != fixName {
		t.Fatalf("fixes: %+v", fixes)
	}
}

func TestMigrationHistory_RenameModifiedNoFix(t *testing.T) {
	body := "create table a();"
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": body,
	})
	f.remove(t, "internal/migrate/sql/001_init.up.sql")
	f.write(t, "internal/migrate/sql/001_init_v2.up.sql", "create table a (id int);")

	notes, fixes, err := f.run(t)
	if err == nil {
		t.Fatalf("expected error, got notes=%+v fixes=%+v", notes, fixes)
	}
	server8 := sha8(body)
	want := "001_init.up-" + server8 + "-<dev8>.sql"
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q missing %q", err.Error(), want)
	}
}

func TestMigrationHistory_InplaceModifiedWithFix(t *testing.T) {
	body := "create table a();"
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": body,
	})
	newBody := "create table a (id int);"
	f.write(t, "internal/migrate/sql/001_init.up.sql", newBody)
	server8 := sha8(body)
	dev8 := sha8(newBody)
	fixName := "001_init.up-" + server8 + "-" + dev8 + ".sql"
	f.write(t, "internal/migrate/sql/migration-fixes/"+fixName, "-- fix")

	notes, fixes, err := f.run(t)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(notes) != 1 || notes[0].Kind != NoteModified {
		t.Fatalf("notes: %+v", notes)
	}
	if len(fixes) != 1 || filepath.Base(fixes[0].Path) != fixName {
		t.Fatalf("fixes: %+v", fixes)
	}
}

func TestMigrationHistory_InplaceModifiedNoFix(t *testing.T) {
	body := "create table a();"
	f := newFixture(t, map[string]string{
		"internal/migrate/sql/001_init.up.sql": body,
	})
	newBody := "create table a (id int);"
	f.write(t, "internal/migrate/sql/001_init.up.sql", newBody)

	notes, fixes, err := f.run(t)
	if err == nil {
		t.Fatalf("expected error, got notes=%+v fixes=%+v", notes, fixes)
	}
	server8 := sha8(body)
	dev8 := sha8(newBody)
	want := filepath.Join("internal/migrate/sql/migration-fixes", "001_init.up-"+server8+"-"+dev8+".sql")
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q missing %q", err.Error(), want)
	}
}
