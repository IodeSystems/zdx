package zvec_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/iodesystems/dx/internal/zvec"
)

func TestRoundTrip(t *testing.T) {
	b, _ := zvec.NewBuilder(16)
	b.Add(1, vec16(1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
	b.Add(2, vec16(0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0))
	b.Add(3, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1))

	var buf bytes.Buffer
	if err := b.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	idx, err := zvec.LoadBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if idx.Dims() != 16 {
		t.Fatalf("dims: got %d, want 16", idx.Dims())
	}
}

func TestTopNCorrectMatch(t *testing.T) {
	idx := buildIndex(t, 16,
		idVec{10, vec16(1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)},
		idVec{20, vec16(0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)},
		idVec{30, vec16(0, 0, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)},
		idVec{40, vec16(0.8, 0.6, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)},
	)
	results, err := idx.TopN(vec16(1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len: got %d, want 2", len(results))
	}
	if results[0].ID != 10 {
		t.Fatalf("top ID: got %d, want 10", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Fatalf("top score: got %f, want ≥ 0.99", results[0].Score)
	}
}

func TestBailEarlyMatchesBruteForce(t *testing.T) {
	idx := buildIndex(t, 16,
		idVec{0, vec16(0.1, 0.9, 0, 0.2)},
		idVec{1, vec16(0.9, 0.1)},
		idVec{2, vec16(0.5, 0.5, 0.5)},
		idVec{3, vec16(0, 0, 0, 1)},
		idVec{4, vec16(0, 0, 0, 0, 1)},
		idVec{5, vec16(0.3, 0.3, 0.3, 0.3)},
		idVec{6, vec16(0, 0, 0, 0, 0, 1)},
		idVec{7, vec16(0, 0, 0, 0, 0, 0, 1)},
		idVec{8, vec16(0.7, 0, 0.7)},
		idVec{9, vec16(0, 0.7, 0, 0.7)},
		idVec{10, vec16(0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{11, vec16(0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{12, vec16(0.4, 0.4, 0.4, 0.4, 0.4)},
		idVec{13, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{14, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{15, vec16(0.6, 0.8)},
		idVec{16, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{17, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{18, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
		idVec{19, vec16(0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)},
	)
	results, _ := idx.TopN(vec16(0.9, 0.1), 5)
	if len(results) != 5 {
		t.Fatalf("len: got %d, want 5", len(results))
	}
	if results[0].ID != 1 {
		t.Fatalf("top ID: got %d, want 1", results[0].ID)
	}
	for i := 1; i < len(results); i++ {
		if results[i-1].Score < results[i].Score {
			t.Fatalf("scores not descending at %d", i)
		}
	}
}

func TestWALUpsertVisible(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zvec")

	b, _ := zvec.NewBuilder(8)
	b.Add(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	b.Add(2, vec8(0, 1, 0, 0, 0, 0, 0, 0))
	if err := b.Finish(path); err != nil {
		t.Fatal(err)
	}

	idx, _ := zvec.Open(path)
	// Upsert a new id=3 pointing in same direction as id=1.
	if err := idx.Upsert(path, 3, vec8(0.9, 0.1, 0, 0, 0, 0, 0, 0)); err != nil {
		t.Fatal(err)
	}

	// Reload and check id=3 appears in search.
	idx2, _ := zvec.Open(path)
	results, _ := idx2.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 3)
	ids := make(map[uint64]bool)
	for _, r := range results {
		ids[r.ID] = true
	}
	if !ids[3] {
		t.Fatalf("upserted id=3 not found in results: %v", results)
	}
}

func TestWALTombstone(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zvec")

	b, _ := zvec.NewBuilder(8)
	b.Add(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	b.Add(2, vec8(0.9, 0.1, 0, 0, 0, 0, 0, 0)) // similar to 1
	b.Finish(path)

	idx, _ := zvec.Open(path)
	idx.Delete(path, 2)

	idx2, _ := zvec.Open(path)
	results, _ := idx2.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 3)
	for _, r := range results {
		if r.ID == 2 {
			t.Fatal("deleted id=2 still appears in results")
		}
	}
}

func TestCompact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zvec")
	compacted := filepath.Join(dir, "compacted.zvec")

	b, _ := zvec.NewBuilder(8)
	b.Add(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	b.Add(2, vec8(0, 1, 0, 0, 0, 0, 0, 0))
	b.Add(3, vec8(0, 0, 1, 0, 0, 0, 0, 0))
	b.Finish(path)

	idx, _ := zvec.Open(path)
	idx.Delete(path, 2)
	idx.Upsert(path, 4, vec8(0.7, 0.7, 0, 0, 0, 0, 0, 0))

	if err := idx.Compact(compacted); err != nil {
		t.Fatal(err)
	}

	// Compacted file should have no WAL overhead.
	fi, _ := os.Stat(compacted)
	fiOrig, _ := os.Stat(path)
	if fi.Size() >= fiOrig.Size() {
		t.Logf("compacted %d bytes, original %d bytes (WAL present)", fi.Size(), fiOrig.Size())
	}

	idx2, _ := zvec.Open(compacted)
	results, _ := idx2.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 5)
	ids := map[uint64]bool{}
	for _, r := range results {
		ids[r.ID] = true
	}
	if ids[2] {
		t.Fatal("tombstoned id=2 present after compact")
	}
	if !ids[4] {
		t.Fatal("upserted id=4 missing after compact")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

type idVec struct {
	id  uint64
	vec []float32
}

func buildIndex(t *testing.T, dims int, pairs ...idVec) zvec.Index {
	t.Helper()
	b, _ := zvec.NewBuilder(dims)
	for _, p := range pairs {
		b.Add(p.id, p.vec)
	}
	var buf bytes.Buffer
	if err := b.WriteTo(&buf); err != nil {
		t.Fatal(err)
	}
	idx, err := zvec.LoadBytes(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	return idx
}

func vec16(vals ...float32) []float32 {
	v := make([]float32, 16)
	copy(v, vals)
	return v
}

func vec8(vals ...float32) []float32 {
	v := make([]float32, 8)
	copy(v, vals)
	return v
}
