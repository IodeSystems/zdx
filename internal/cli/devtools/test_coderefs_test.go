package devtools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadCodeRefSidecars_HappyPath(t *testing.T) {
	tmp := t.TempDir()
	body := `[{"file_path":"a.go","line_start":1,"line_end":2,"note":"x"}]`
	if err := os.WriteFile(filepath.Join(tmp, "TestA.json"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := readCodeRefSidecars(tmp)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	refs, ok := out["TestA"]
	if !ok {
		t.Fatalf("expected key TestA, got keys %v", keys(out))
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	r := refs[0]
	if r.FilePath != "a.go" || r.LineStart != 1 || r.LineEnd != 2 || r.Note != "x" {
		t.Fatalf("unexpected ref: %+v", r)
	}
}

func TestReadCodeRefSidecars_SkipsNonJSONAndDirs(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "TestA.json"), []byte(`[{"file_path":"a.go"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "notes.txt"), []byte("ignore"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmp, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "nested", "TestB.json"), []byte(`[{"file_path":"b.go"}]`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "bad.json"), []byte("not-json"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "empty.json"), []byte(`[]`), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := readCodeRefSidecars(tmp)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 entry, got %d: %v", len(out), keys(out))
	}
	if _, ok := out["TestA"]; !ok {
		t.Fatalf("expected TestA, got %v", keys(out))
	}
}

func TestReadCodeRefSidecars_MissingDir(t *testing.T) {
	out, err := readCodeRefSidecars(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected nil err for missing dir, got %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil map, got %v", out)
	}
}

func keys(m map[string][]codeRefSidecar) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
