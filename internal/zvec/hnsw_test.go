package zvec_test

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/iodesystems/zdx-go/internal/zvec"
)

func TestHNSWBasicTopN(t *testing.T) {
	h, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4, EfConstruct: 20})
	h.Insert(10, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	h.Insert(20, vec8(0, 1, 0, 0, 0, 0, 0, 0))
	h.Insert(30, vec8(0, 0, 1, 0, 0, 0, 0, 0))
	h.Insert(40, vec8(0.8, 0.6, 0, 0, 0, 0, 0, 0))

	results, err := h.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].ID != 10 {
		t.Fatalf("top result: got id=%d, want 10", results[0].ID)
	}
	if results[0].Score < 0.99 {
		t.Fatalf("top score %f < 0.99", results[0].Score)
	}
}

func TestHNSWSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.hnsw")

	h, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4, EfConstruct: 20})
	for i, v := range [][]float32{
		{1, 0, 0, 0, 0, 0, 0, 0},
		{0, 1, 0, 0, 0, 0, 0, 0},
		{0, 0, 1, 0, 0, 0, 0, 0},
		{0.8, 0.6, 0, 0, 0, 0, 0, 0},
	} {
		h.Insert(uint64(i+1), v)
	}
	if err := h.Save(path); err != nil {
		t.Fatal(err)
	}

	h2, err := zvec.LoadHNSW(path)
	if err != nil {
		t.Fatal(err)
	}
	results, err := h2.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].ID != 1 {
		t.Fatalf("after reload: top id=%v", results)
	}
}

func TestHNSWOpenDetectsType(t *testing.T) {
	dir := t.TempDir()
	hnswPath := filepath.Join(dir, "a.hnsw")
	zvecPath := filepath.Join(dir, "b.zvec")

	h, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4})
	h.Insert(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	h.Save(hnswPath)

	b, _ := zvec.NewBuilder(8)
	b.Add(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	b.Finish(zvecPath)

	for _, path := range []string{hnswPath, zvecPath} {
		idx, err := zvec.Open(path)
		if err != nil {
			t.Fatalf("Open(%s): %v", path, err)
		}
		results, _ := idx.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 1)
		if len(results) == 0 || results[0].ID != 1 {
			t.Fatalf("Open(%s): unexpected results %v", path, results)
		}
	}
}

func TestHNSWScaleTopN(t *testing.T) {
	// 200 vectors: query should return correct top match.
	// EfConstruct=400 ensures thorough graph construction for reliable exact-match retrieval.
	h, _ := zvec.NewHNSW(16, zvec.HNSWOptions{M: 16, EfConstruct: 400})

	target := vec16(1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0)
	h.Insert(999, target)

	for i := 1; i < 200; i++ {
		v := make([]float32, 16)
		v[i%16] = 1
		v[(i+1)%16] = 0.3
		h.Insert(uint64(i), v)
	}

	results, err := h.TopN(target, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("no results")
	}
	if results[0].ID != 999 {
		t.Fatalf("expected id=999 at top, got id=%d score=%.4f", results[0].ID, results[0].Score)
	}
}

func TestHNSWUpsertDelete(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.hnsw")

	h, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4, EfConstruct: 20})
	h.Insert(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	h.Insert(2, vec8(0, 1, 0, 0, 0, 0, 0, 0))
	h.Save(path)

	h2, _ := zvec.LoadHNSW(path)
	h2.Upsert(path, 3, vec8(0.9, 0.1, 0, 0, 0, 0, 0, 0))
	h2.Delete(path, 2)

	h3, _ := zvec.LoadHNSW(path)
	results, _ := h3.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 5)
	ids := map[uint64]bool{}
	for _, r := range results {
		ids[r.ID] = true
	}
	if ids[2] {
		t.Fatal("deleted id=2 still present")
	}
	if !ids[3] {
		t.Fatalf("upserted id=3 missing; results: %v", results)
	}
}

func TestHNSWCompact(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.hnsw")
	compacted := filepath.Join(dir, "compact.hnsw")

	h, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4, EfConstruct: 20})
	h.Insert(1, vec8(1, 0, 0, 0, 0, 0, 0, 0))
	h.Insert(2, vec8(0, 1, 0, 0, 0, 0, 0, 0))
	h.Insert(3, vec8(0, 0, 1, 0, 0, 0, 0, 0))
	h.Save(path)

	h2, _ := zvec.LoadHNSW(path)
	h2.Delete(path, 2)
	h2.Upsert(path, 4, vec8(0.7, 0.7, 0, 0, 0, 0, 0, 0))
	h2.Compact(compacted)

	h3, _ := zvec.LoadHNSW(compacted)
	results, _ := h3.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 5)
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

// ── Interface interop ─────────────────────────────────────────────────────────

func TestInterfaceInterop(t *testing.T) {
	// Both FlatIndex and HNSWIndex satisfy zvec.Index.
	// Build both with same data, expect same top result.
	vecs := []struct {
		id  uint64
		vec []float32
	}{
		{1, vec8(1, 0, 0, 0, 0, 0, 0, 0)},
		{2, vec8(0, 1, 0, 0, 0, 0, 0, 0)},
		{3, vec8(0, 0, 1, 0, 0, 0, 0, 0)},
		{4, vec8(0.8, 0.6, 0, 0, 0, 0, 0, 0)},
	}

	fb, _ := zvec.NewBuilder(8)
	hb, _ := zvec.NewHNSW(8, zvec.HNSWOptions{M: 4, EfConstruct: 20})
	for _, v := range vecs {
		fb.Add(v.id, v.vec)
		hb.Insert(v.id, v.vec)
	}

	var buf bytes.Buffer
	fb.Serialize(&buf)
	flat, _ := zvec.LoadBytes(buf.Bytes())

	for _, idx := range []zvec.Index{flat, hb} {
		results, err := idx.TopN(vec8(1, 0, 0, 0, 0, 0, 0, 0), 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 || results[0].ID != 1 {
			t.Fatalf("%T: expected id=1 at top, got %v", idx, results)
		}
	}
}
