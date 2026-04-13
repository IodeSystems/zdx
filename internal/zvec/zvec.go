// Package zvec implements a cosine-similarity flat index compatible with the
// Zig zvec binary format (.zvec files).
//
// File layout (little-endian):
//
//	Header      20 bytes  magic "ZVEC", version, lane_width, pad, dimensions, count
//	dim_order   D × u32   permutation: permuted[i] = original[dim_order[i]]
//	Records × count:
//	  id          u64
//	  vector      D × f32   permuted + unit-normalized
//	  tail_norms  (D/8 - 1) × f32
//	WAL entries (zero or more, appended after main records):
//	  type        u8        1=upsert, 2=tombstone
//	  id          u64
//	  (upsert only) vector D × f32, tail_norms (D/8-1) × f32
//
// Search uses Cauchy-Schwarz early termination: after each SIMD chunk we check
// whether the remaining dimensions can beat the current nth-best score.
// Dimensions are sorted by corpus variance descending so the bound tightens
// quickly for dissimilar vectors.
//
// Mutations (Upsert/Delete) append WAL entries to the file; Compact rewrites
// a clean index from the live set.
package zvec

import (
	"container/heap"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

const (
	magic       = uint32('Z' | 'V'<<8 | 'E'<<16 | 'C'<<24)
	currentVer  = uint8(1)
	lane        = 8 // AVX2: 8 × f32
	headerBytes = 20

	walUpsert    = uint8(1)
	walTombstone = uint8(2)
)

// ── Header ────────────────────────────────────────────────────────────────────

type header struct {
	Magic      uint32
	Version    uint8
	LaneWidth  uint8
	Pad        uint16
	Dimensions uint32
	Count      uint64
}

// ── Index ─────────────────────────────────────────────────────────────────────

// Index is a loaded, searchable zvec index held entirely in memory.
type FlatIndex struct {
	hdr      header
	dimOrder []uint32
	// main records
	ids     []uint64
	vectors []float32 // len = N*D
	tails   []float32 // len = N*(D/lane-1)
	// WAL overlay: last-write-wins per ID.
	// walLatest maps id → walEntry; tombstones have vec==nil.
	walLatest map[uint64]walEntry
}

type walEntry struct {
	vec   []float32 // nil = tombstone
	tails []float32
}

// ── Load ──────────────────────────────────────────────────────────────────────

// Load reads a .zvec file from path, including any WAL entries.
func Load(path string) (*FlatIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readAll(f)
}

// LoadBytes reads a .zvec index from a byte slice.
func LoadBytes(data []byte) (*FlatIndex, error) {
	return readAll(&byteReader{data: data})
}

func readAll(r io.Reader) (*FlatIndex, error) {
	var hdr header
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("zvec: read header: %w", err)
	}
	if hdr.Magic != magic {
		return nil, fmt.Errorf("zvec: bad magic %08x", hdr.Magic)
	}
	if hdr.Version != currentVer {
		return nil, fmt.Errorf("zvec: unsupported version %d", hdr.Version)
	}
	if hdr.LaneWidth != lane {
		return nil, fmt.Errorf("zvec: lane width %d (expected %d)", hdr.LaneWidth, lane)
	}
	d := int(hdr.Dimensions)
	n := int(hdr.Count)
	if d == 0 || d%lane != 0 {
		return nil, fmt.Errorf("zvec: bad dimensions %d", d)
	}
	tailCount := d/lane - 1

	dimOrder := make([]uint32, d)
	if err := binary.Read(r, binary.LittleEndian, dimOrder); err != nil {
		return nil, fmt.Errorf("zvec: read dim_order: %w", err)
	}

	ids := make([]uint64, n)
	vectors := make([]float32, n*d)
	tails := make([]float32, n*tailCount)

	for i := 0; i < n; i++ {
		if err := binary.Read(r, binary.LittleEndian, &ids[i]); err != nil {
			return nil, fmt.Errorf("zvec: record %d id: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, vectors[i*d:(i+1)*d]); err != nil {
			return nil, fmt.Errorf("zvec: record %d vector: %w", i, err)
		}
		if tailCount > 0 {
			if err := binary.Read(r, binary.LittleEndian, tails[i*tailCount:(i+1)*tailCount]); err != nil {
				return nil, fmt.Errorf("zvec: record %d tails: %w", i, err)
			}
		}
	}

	// Read WAL entries until EOF.
	walLatest := map[uint64]walEntry{}
	for {
		var typ uint8
		if err := binary.Read(r, binary.LittleEndian, &typ); err != nil {
			break // EOF or truncated — stop
		}
		var id uint64
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			break
		}
		switch typ {
		case walTombstone:
			walLatest[id] = walEntry{}
		case walUpsert:
			vec := make([]float32, d)
			if err := binary.Read(r, binary.LittleEndian, vec); err != nil {
				goto doneWAL
			}
			var t []float32
			if tailCount > 0 {
				t = make([]float32, tailCount)
				if err := binary.Read(r, binary.LittleEndian, t); err != nil {
					goto doneWAL
				}
			}
			walLatest[id] = walEntry{vec: vec, tails: t}
		default:
			goto doneWAL // unknown type — stop reading
		}
	}
doneWAL:

	return &FlatIndex{
		hdr: hdr, dimOrder: dimOrder,
		ids: ids, vectors: vectors, tails: tails,
		walLatest: walLatest,
	}, nil
}

// Dims returns the dimensionality of stored vectors.
func (idx *FlatIndex) Dims() int { return int(idx.hdr.Dimensions) }

// Count returns the number of live vectors (main records minus tombstones plus WAL upserts for new IDs).
func (idx *FlatIndex) Count() int {
	// Count main records that are still live.
	live := 0
	seen := map[uint64]bool{}
	for _, id := range idx.ids {
		if e, ok := idx.walLatest[id]; ok {
			if e.vec == nil {
				continue // tombstoned
			}
			if !seen[id] {
				seen[id] = true
				live++ // will be counted via WAL path
			}
			continue
		}
		live++
	}
	// Add WAL upserts for IDs not in main records.
	mainIDs := map[uint64]bool{}
	for _, id := range idx.ids {
		mainIDs[id] = true
	}
	for id, e := range idx.walLatest {
		if !mainIDs[id] && e.vec != nil {
			live++
		}
	}
	return live
}

// ── Mutations ─────────────────────────────────────────────────────────────────

// Upsert appends a WAL upsert entry to path for the given id and raw vector.
// The vector is permuted and normalized using the index's dim_order before writing.
func (idx *FlatIndex) Upsert(path string, id uint64, rawVec []float32) error {
	d := idx.Dims()
	if len(rawVec) != d {
		return fmt.Errorf("zvec: dimension mismatch: got %d, want %d", len(rawVec), d)
	}
	tailCount := d/lane - 1

	permuted := make([]float32, d)
	for i, orig := range idx.dimOrder {
		permuted[i] = rawVec[orig]
	}
	normalizeVec(permuted)
	var tailBuf []float32
	if tailCount > 0 {
		tailBuf = make([]float32, tailCount)
		tailNorms(permuted, tailBuf)
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, walUpsert); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, id); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, permuted); err != nil {
		return err
	}
	if tailCount > 0 {
		if err := binary.Write(f, binary.LittleEndian, tailBuf); err != nil {
			return err
		}
	}

	// Update in-memory WAL.
	idx.walLatest[id] = walEntry{vec: permuted, tails: tailBuf}
	return nil
}

// Delete appends a WAL tombstone for id to path.
func (idx *FlatIndex) Delete(path string, id uint64) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	if err := binary.Write(f, binary.LittleEndian, walTombstone); err != nil {
		return err
	}
	if err := binary.Write(f, binary.LittleEndian, id); err != nil {
		return err
	}

	idx.walLatest[id] = walEntry{}
	return nil
}

// Compact rewrites path as a clean index with no WAL, incorporating all live
// WAL upserts and dropping tombstoned entries.
func (idx *FlatIndex) Compact(path string) error {
	d := idx.Dims()

	b := &Builder{
		dims:  d,
		sum:   make([]float64, d),
		sumSq: make([]float64, d),
	}

	// Add live main records (already permuted+normalized — re-use as-is via
	// a raw add that bypasses permutation).
	mainIDs := map[uint64]bool{}
	for i, id := range idx.ids {
		mainIDs[id] = true
		e, overridden := idx.walLatest[id]
		if overridden {
			if e.vec == nil {
				continue // tombstoned
			}
			// Will be added via WAL path below.
			continue
		}
		// Re-add raw vector — we only have the permuted+normalized form, so
		// we add it as the "original" (sum/sumSq stats will be wrong for
		// variance-sort purposes but the vectors themselves are already optimal).
		b.entries = append(b.entries, entry{id: id, vector: append([]float32{}, idx.vectors[i*d:(i+1)*d]...)})
	}

	// Add WAL upserts (new IDs and overrides).
	for id, e := range idx.walLatest {
		if e.vec == nil {
			continue // tombstone
		}
		b.entries = append(b.entries, entry{id: id, vector: append([]float32{}, e.vec...)})
	}

	// For compact, preserve the existing dim_order (variance already computed
	// at build time; WAL entries were already permuted with it).
	// Write directly rather than going through Builder.Serialize which would
	// re-sort dims — instead write with the existing dim_order.
	return idx.writeCompact(path, b.entries)
}

func (idx *FlatIndex) writeCompact(path string, entries []entry) error {
	d := idx.Dims()
	tailCount := d/lane - 1

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	hdr := header{
		Magic:      magic,
		Version:    currentVer,
		LaneWidth:  lane,
		Dimensions: uint32(d),
		Count:      uint64(len(entries)),
	}
	if err := binary.Write(f, binary.LittleEndian, &hdr); err != nil {
		return err
	}
	for _, orig := range idx.dimOrder {
		if err := binary.Write(f, binary.LittleEndian, orig); err != nil {
			return err
		}
	}

	tailBuf := make([]float32, tailCount)
	for _, e := range entries {
		if tailCount > 0 {
			tailNorms(e.vector, tailBuf)
		}
		if err := binary.Write(f, binary.LittleEndian, e.id); err != nil {
			return err
		}
		if err := binary.Write(f, binary.LittleEndian, e.vector); err != nil {
			return err
		}
		if tailCount > 0 {
			if err := binary.Write(f, binary.LittleEndian, tailBuf); err != nil {
				return err
			}
		}
	}
	return nil
}

// ── Builder ───────────────────────────────────────────────────────────────────

type entry struct {
	id     uint64
	vector []float32
}

// Builder accumulates vectors and writes a .zvec file.
type Builder struct {
	dims    int
	entries []entry
	sum     []float64
	sumSq   []float64
}

// NewBuilder creates a builder for vectors of the given dimensionality.
// dims must be a positive multiple of 8.
func NewBuilder(dims int) (*Builder, error) {
	if dims <= 0 || dims%lane != 0 {
		return nil, fmt.Errorf("zvec: dims must be a positive multiple of %d, got %d", lane, dims)
	}
	return &Builder{
		dims:  dims,
		sum:   make([]float64, dims),
		sumSq: make([]float64, dims),
	}, nil
}

// Add copies and records a raw (unnormalized) embedding with the given id.
func (b *Builder) Add(id uint64, vec []float32) error {
	if len(vec) != b.dims {
		return fmt.Errorf("zvec: dimension mismatch: got %d, want %d", len(vec), b.dims)
	}
	v := make([]float32, b.dims)
	copy(v, vec)
	b.entries = append(b.entries, entry{id: id, vector: v})
	for i, x := range vec {
		fx := float64(x)
		b.sum[i] += fx
		b.sumSq[i] += fx * fx
	}
	return nil
}

// WriteTo serializes the index to w.
func (b *Builder) Serialize(w io.Writer) error {
	n := len(b.entries)
	d := b.dims
	chunks := d / lane
	tailCount := chunks - 1

	dimOrder := make([]int, d)
	for i := range dimOrder {
		dimOrder[i] = i
	}
	variance := make([]float64, d)
	fn := float64(maxInt(n, 1))
	for i := range d {
		mean := b.sum[i] / fn
		variance[i] = b.sumSq[i]/fn - mean*mean
	}
	sort.Slice(dimOrder, func(a, c int) bool {
		return variance[dimOrder[a]] > variance[dimOrder[c]]
	})

	hdr := header{
		Magic:      magic,
		Version:    currentVer,
		LaneWidth:  lane,
		Dimensions: uint32(d),
		Count:      uint64(n),
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}
	for _, idx := range dimOrder {
		if err := binary.Write(w, binary.LittleEndian, uint32(idx)); err != nil {
			return err
		}
	}

	permuted := make([]float32, d)
	tailBuf := make([]float32, tailCount)

	for _, e := range b.entries {
		for i, orig := range dimOrder {
			permuted[i] = e.vector[orig]
		}
		normalizeVec(permuted)
		if tailCount > 0 {
			tailNorms(permuted, tailBuf)
		}
		if err := binary.Write(w, binary.LittleEndian, e.id); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, permuted); err != nil {
			return err
		}
		if tailCount > 0 {
			if err := binary.Write(w, binary.LittleEndian, tailBuf); err != nil {
				return err
			}
		}
	}
	return nil
}

// Finish writes the index to path.
func (b *Builder) Finish(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return b.Serialize(f)
}

// ── Search ────────────────────────────────────────────────────────────────────

// SearchResult is a single hit from TopN.
type SearchResult struct {
	ID    uint64
	Score float32
}

// TopN returns the n most similar vectors to queryRaw.
// queryRaw must have the same dimensionality as the index.
// Results are sorted by score descending. WAL upserts and tombstones are
// applied; last-write-wins per ID.
func (idx *FlatIndex) TopN(queryRaw []float32, n int) ([]SearchResult, error) {
	d := idx.Dims()
	if len(queryRaw) != d {
		return nil, fmt.Errorf("zvec: query dim %d != index dim %d", len(queryRaw), d)
	}
	if n <= 0 {
		return nil, nil
	}
	chunks := d / lane
	tailCount := chunks - 1

	q := make([]float32, d)
	for i, orig := range idx.dimOrder {
		q[i] = queryRaw[orig]
	}
	normalizeVec(q)

	qtail := make([]float32, tailCount)
	if tailCount > 0 {
		tailNorms(q, qtail)
	}

	h := &resultHeap{}
	heap.Init(h)
	nthBest := float32(math.Inf(-1))

	score := func(vec, vtail []float32) float32 {
		var partial float32
		for ck := 0; ck < chunks; ck++ {
			base := ck * lane
			for j := 0; j < lane; j++ {
				partial += q[base+j] * vec[base+j]
			}
			if ck < tailCount {
				if partial+qtail[ck]*vtail[ck] < nthBest {
					return math.SmallestNonzeroFloat32 - 1 // sentinel: bailed
				}
			}
		}
		return partial
	}

	push := func(id uint64, s float32) {
		if s <= math.SmallestNonzeroFloat32-1 {
			return
		}
		if h.Len() < n {
			heap.Push(h, SearchResult{ID: id, Score: s})
			if h.Len() == n {
				nthBest = (*h)[0].Score
			}
		} else if s > nthBest {
			heap.Pop(h)
			heap.Push(h, SearchResult{ID: id, Score: s})
			nthBest = (*h)[0].Score
		}
	}

	// Scan main records.
	seenInWAL := idx.walLatest // non-nil map means id has a WAL entry
	for i, id := range idx.ids {
		if e, ok := seenInWAL[id]; ok {
			if e.vec == nil {
				continue // tombstoned
			}
			// Overridden by WAL — skip here; will score via WAL pass.
			continue
		}
		vec := idx.vectors[i*d : (i+1)*d]
		var vt []float32
		if tailCount > 0 {
			vt = idx.tails[i*tailCount : (i+1)*tailCount]
		}
		push(id, score(vec, vt))
	}

	// Scan WAL upserts.
	for id, e := range seenInWAL {
		if e.vec == nil {
			continue
		}
		push(id, score(e.vec, e.tails))
	}

	results := make([]SearchResult, h.Len())
	j := len(results)
	for h.Len() > 0 {
		j--
		results[j] = heap.Pop(h).(SearchResult)
	}
	return results, nil
}

// ── SIMD helpers ──────────────────────────────────────────────────────────────

func normalizeVec(v []float32) {
	var sq float32
	for _, x := range v {
		sq += x * x
	}
	if sq < 1e-20 {
		return
	}
	inv := float32(1.0 / math.Sqrt(float64(sq)))
	for i := range v {
		v[i] *= inv
	}
}

func tailNorms(v []float32, out []float32) {
	chunks := len(v) / lane
	var sq float32
	for j := chunks - 1; j >= 1; j-- {
		base := j * lane
		for k := 0; k < lane; k++ {
			sq += v[base+k] * v[base+k]
		}
		out[j-1] = float32(math.Sqrt(float64(sq)))
	}
}

// ── Heap ──────────────────────────────────────────────────────────────────────

type resultHeap []SearchResult

func (h resultHeap) Len() int            { return len(h) }
func (h resultHeap) Less(i, j int) bool  { return h[i].Score < h[j].Score }
func (h resultHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *resultHeap) Push(x any)         { *h = append(*h, x.(SearchResult)) }
func (h *resultHeap) Pop() any           { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

// ── byteReader ────────────────────────────────────────────────────────────────

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
