package migratecmd

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// writePair creates an up.sql + down.sql pair with placeholder content.
func writePair(t *testing.T, dir string, nnn int, name string) {
	t.Helper()
	for _, suf := range []string{"up", "down"} {
		path := filepath.Join(dir, formatN(nnn)+"_"+name+"."+suf+".sql")
		if err := os.WriteFile(path, []byte("-- "+suf+"\n"), 0644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
}

func formatN(n int) string {
	return formatN3(n)
}

// formatN3 mirrors the production "%03d" format.
func formatN3(n int) string {
	if n < 10 {
		return "00" + itoa(n)
	}
	if n < 100 {
		return "0" + itoa(n)
	}
	return itoa(n)
}

func itoa(n int) string {
	const digits = "0123456789"
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{digits[n%10]}, b...)
		n /= 10
	}
	return string(b)
}

func listSQL(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// TestRenumber_AutoCollision: 001..005 + colliding 003_other -> 006_other.
func TestRenumber_AutoCollision(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 5; i++ {
		writePair(t, dir, i, "step")
	}
	writePair(t, dir, 3, "other")

	var out bytes.Buffer
	if err := runRenumber(dir, true, &out, strings.NewReader("")); err != nil {
		t.Fatalf("runRenumber: %v", err)
	}

	got := listSQL(t, dir)
	// "other" < "step" alphabetically, so "other" keeps slot 003 and the
	// step pair previously at 003 moves to next free (006).
	wantPresent := []string{
		"003_other.up.sql",
		"003_other.down.sql",
		"006_step.up.sql",
		"006_step.down.sql",
	}
	for _, w := range wantPresent {
		found := false
		for _, g := range got {
			if g == w {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing expected file %s; got: %v", w, got)
		}
	}
	for _, g := range got {
		if g == "003_step.up.sql" || g == "003_step.down.sql" {
			t.Errorf("colliding file %s still present", g)
		}
	}
	if !strings.Contains(out.String(), "renamed 1 pair") {
		t.Errorf("expected 'renamed 1 pair' in output; got: %s", out.String())
	}
}

// TestRenumber_NoCollision: clean dir reports nothing and changes nothing.
func TestRenumber_NoCollision(t *testing.T) {
	dir := t.TempDir()
	for i := 1; i <= 3; i++ {
		writePair(t, dir, i, "step")
	}
	before := listSQL(t, dir)

	var out bytes.Buffer
	if err := runRenumber(dir, true, &out, strings.NewReader("")); err != nil {
		t.Fatalf("runRenumber: %v", err)
	}

	after := listSQL(t, dir)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("files changed unexpectedly: before=%v after=%v", before, after)
	}
	if !strings.Contains(out.String(), "no collisions") {
		t.Errorf("expected 'no collisions'; got: %s", out.String())
	}
}

// TestRenumber_PromptDecline: without --auto, 'n' aborts; no rename.
func TestRenumber_PromptDecline(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "first")
	writePair(t, dir, 1, "second")
	before := listSQL(t, dir)

	var out bytes.Buffer
	if err := runRenumber(dir, false, &out, strings.NewReader("n\n")); err != nil {
		t.Fatalf("runRenumber: %v", err)
	}

	after := listSQL(t, dir)
	if strings.Join(before, ",") != strings.Join(after, ",") {
		t.Errorf("files changed despite decline: before=%v after=%v", before, after)
	}
	if !strings.Contains(out.String(), "aborted") {
		t.Errorf("expected 'aborted'; got: %s", out.String())
	}
}

// TestRenumber_PromptAccept: without --auto, 'y' performs the rename.
func TestRenumber_PromptAccept(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 1, "alpha")
	writePair(t, dir, 1, "bravo")

	var out bytes.Buffer
	if err := runRenumber(dir, false, &out, strings.NewReader("y\n")); err != nil {
		t.Fatalf("runRenumber: %v", err)
	}

	got := listSQL(t, dir)
	// alpha < bravo lexicographically; alpha keeps slot 1, bravo moves to 2.
	want := map[string]bool{
		"001_alpha.up.sql":   true,
		"001_alpha.down.sql": true,
		"002_bravo.up.sql":   true,
		"002_bravo.down.sql": true,
	}
	if len(got) != len(want) {
		t.Fatalf("file count mismatch: got %v", got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected file %s; got: %v", g, got)
		}
	}
}

// TestRenumber_MultipleCollisions: two distinct collisions get distinct slots.
func TestRenumber_MultipleCollisions(t *testing.T) {
	dir := t.TempDir()
	writePair(t, dir, 5, "a")
	writePair(t, dir, 5, "b") // collision at 5
	writePair(t, dir, 5, "c") // another collision at 5
	writePair(t, dir, 7, "d")
	writePair(t, dir, 7, "e") // collision at 7

	var out bytes.Buffer
	if err := runRenumber(dir, true, &out, strings.NewReader("")); err != nil {
		t.Fatalf("runRenumber: %v", err)
	}

	// max NNN is 7, next free starts at 8. Collisions processed in NNN order
	// (5 first, then 7). Within each, alphabetically-first stays.
	// 5: a stays, b -> 8, c -> 9. 7: d stays, e -> 10.
	got := listSQL(t, dir)
	want := []string{
		"005_a.down.sql", "005_a.up.sql",
		"007_d.down.sql", "007_d.up.sql",
		"008_b.down.sql", "008_b.up.sql",
		"009_c.down.sql", "009_c.up.sql",
		"010_e.down.sql", "010_e.up.sql",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("files mismatch:\n got:  %v\n want: %v", got, want)
	}
}

// TestPlanRenumber_Empty: zero files -> no plans.
func TestPlanRenumber_Empty(t *testing.T) {
	if got := planRenumber(nil); len(got) != 0 {
		t.Errorf("expected no plans; got %v", got)
	}
}
