package zvec

// HNSW — Hierarchical Navigable Small World approximate nearest-neighbor index.
//
// File layout (little-endian):
//
//	Header (40 bytes):
//	  magic      u32  "HNSW"
//	  version    u8   1
//	  lane_width u8
//	  pad        u16
//	  dimensions u32
//	  m          u32  max connections per node per layer (typ. 16)
//	  m0         u32  max connections at layer 0 (typ. 2*M)
//	  count      u64
//	  entry_idx  u64  index of entry point (^0 if empty)
//	  entry_layer u32
//	  pad2       u32
//	Nodes × count:
//	  id         u64
//	  max_layer  u32
//	  pad        u32
//	  vector     dimensions × f32  (unit-normalized)
//	  tails      (dimensions/lane-1) × f32
//	  layer_0_links  m0 × u32  (^0 = empty slot)
//	  layer_k_links  m  × u32  for k = 1..max_layer
//	WAL entries (same format as FlatIndex WAL):
//	  type u8, id u64, (upsert) vector + tails

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
)

const (
	hnswMagic   = uint32('H' | 'N'<<8 | 'S'<<16 | 'W'<<24)
	hnswVersion = uint8(1)
	emptyLink   = uint32(math.MaxUint32)
)

// ── Options ───────────────────────────────────────────────────────────────────

// HNSWOptions tunes the index. Zero value uses defaults.
type HNSWOptions struct {
	M           int // max neighbors per layer (default 16)
	M0          int // max neighbors at layer 0 (default 2*M)
	EfConstruct int // beam width during insert (default 200)
}

func (o HNSWOptions) m() int {
	if o.M <= 0 {
		return 16
	}
	return o.M
}
func (o HNSWOptions) m0() int {
	if o.M0 <= 0 {
		return o.m() * 2
	}
	return o.M0
}
func (o HNSWOptions) efConstruct() int {
	if o.EfConstruct <= 0 {
		return 200
	}
	return o.EfConstruct
}

// ── In-memory node ─────────────────────────────────────────────────────────────

type hnswNode struct {
	id       uint64
	maxLayer int
	vec      []float32 // normalized
	tails    []float32
	links    [][]uint32 // links[layer]; emptyLink = vacant slot
	deleted  bool
}

func (n *hnswNode) liveLinks(layer int) []uint32 {
	out := n.links[layer][:0:len(n.links[layer])]
	for _, l := range n.links[layer] {
		if l != emptyLink {
			out = append(out, l)
		}
	}
	return out
}

// ── HNSWIndex ──────────────────────────────────────────────────────────────────

// HNSWIndex implements approximate nearest-neighbor search.
type HNSWIndex struct {
	dims       int
	opts       HNSWOptions
	nodes      []*hnswNode
	idToIdx    map[uint64]int
	entryIdx   int // -1 if empty
	entryLayer int
	walLatest  map[uint64]walEntry
}

// NewHNSW creates an empty in-memory HNSW index.
func NewHNSW(dims int, opts HNSWOptions) (*HNSWIndex, error) {
	if dims <= 0 || dims%lane != 0 {
		return nil, fmt.Errorf("zvec/hnsw: dims must be positive multiple of %d, got %d", lane, dims)
	}
	return &HNSWIndex{
		dims:      dims,
		opts:      opts,
		idToIdx:   map[uint64]int{},
		entryIdx:  -1,
		walLatest: map[uint64]walEntry{},
	}, nil
}

func (h *HNSWIndex) Dims() int  { return h.dims }
func (h *HNSWIndex) Count() int { return len(h.nodes) }

// ── Insert ─────────────────────────────────────────────────────────────────────

// Insert adds a raw (unnormalized) vector to the in-memory index.
func (h *HNSWIndex) Insert(id uint64, rawVec []float32) error {
	if len(rawVec) != h.dims {
		return fmt.Errorf("zvec/hnsw: dim mismatch: got %d want %d", len(rawVec), h.dims)
	}
	d := h.dims
	tailCount := d/lane - 1
	m, m0 := h.opts.m(), h.opts.m0()

	vec := make([]float32, d)
	copy(vec, rawVec)
	normalizeVec(vec)
	tails := make([]float32, tailCount)
	if tailCount > 0 {
		tailNorms(vec, tails)
	}

	maxLayer := assignLayer(m)
	links := make([][]uint32, maxLayer+1)
	for l := range links {
		cap := m0
		if l > 0 {
			cap = m
		}
		links[l] = make([]uint32, cap)
		for i := range links[l] {
			links[l][i] = emptyLink
		}
	}

	newIdx := len(h.nodes)
	node := &hnswNode{id: id, maxLayer: maxLayer, vec: vec, tails: tails, links: links}
	h.nodes = append(h.nodes, node)
	h.idToIdx[id] = newIdx

	if h.entryIdx == -1 {
		h.entryIdx = newIdx
		h.entryLayer = maxLayer
		return nil
	}

	ef := h.opts.efConstruct()
	ep := []uint32{uint32(h.entryIdx)}

	// From top layer down to maxLayer+1: greedy ef=1 to narrow entry point.
	for lc := h.entryLayer; lc > maxLayer; lc-- {
		ep = h.searchLayer(vec, ep, 1, lc)
	}

	// From min(maxLayer, entryLayer) down to 0: find neighbors, wire links.
	for lc := minInt(maxLayer, h.entryLayer); lc >= 0; lc-- {
		cap := m0
		if lc > 0 {
			cap = m
		}
		candidates := h.searchLayer(vec, ep, ef, lc)
		neighbors := h.selectNeighbors(vec, candidates, cap)

		// Wire new node → neighbors.
		for i, nIdx := range neighbors {
			if i >= len(node.links[lc]) {
				break
			}
			node.links[lc][i] = nIdx
		}

		// Wire neighbors → new node (bidirectional); prune if over cap.
		for _, nIdx := range neighbors {
			nb := h.nodes[nIdx]
			if lc >= len(nb.links) {
				continue
			}
			maxCap := m0
			if lc > 0 {
				maxCap = m
			}
			h.addLink(nb, lc, uint32(newIdx), maxCap)
		}

		ep = candidates
	}

	if maxLayer > h.entryLayer {
		h.entryIdx = newIdx
		h.entryLayer = maxLayer
	}
	return nil
}

func (h *HNSWIndex) addLink(node *hnswNode, layer int, newLink uint32, maxCap int) {
	links := node.links[layer]
	// Find vacant slot.
	for i, l := range links {
		if l == emptyLink {
			links[i] = newLink
			return
		}
	}
	// All slots full: replace the weakest neighbor (furthest from node).
	weakIdx := 0
	weakSim := float32(math.MaxFloat32)
	nv := node.vec
	for i, l := range links {
		sim := dotProduct(nv, h.nodes[l].vec)
		if sim < weakSim {
			weakSim = sim
			weakIdx = i
		}
	}
	// Only replace if new link is stronger.
	newSim := dotProduct(nv, h.nodes[newLink].vec)
	if newSim > weakSim {
		links[weakIdx] = newLink
	}
}

// ── Search layer (Algorithm 2 from the HNSW paper) ────────────────────────────

type searchItem struct {
	idx uint32
	sim float32
}

func sortItems2(items []searchItem) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].sim < key.sim {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}

// searchLayer finds ef nearest neighbors to q starting from entry points ep at layer lc.
// Returns indices of best candidates (sorted by similarity desc).
func (h *HNSWIndex) searchLayer(q []float32, ep []uint32, ef, lc int) []uint32 {
	visited := map[uint32]bool{}

	var cands []searchItem  // min-heap semantics: pop smallest sim
	var result []searchItem // max-heap semantics: pop largest sim (so we can evict worst)

	push := func(idx uint32, sim float32) {
		it := searchItem{idx, sim}
		cands = append(cands, it)
		result = append(result, it)
	}

	for _, e := range ep {
		if !visited[e] && !h.nodes[e].deleted {
			visited[e] = true
			sim := dotProduct(q, h.nodes[e].vec)
			push(e, sim)
		}
	}

	for len(cands) > 0 {
		// Pop best from cands (highest sim not yet processed).
		best := 0
		for i, c := range cands {
			if c.sim > cands[best].sim {
				best = i
			}
		}
		c := cands[best]
		cands[best] = cands[len(cands)-1]
		cands = cands[:len(cands)-1]

		// Worst in result set.
		worstSim := float32(math.Inf(-1))
		if len(result) >= ef {
			for _, r := range result {
				if r.sim > worstSim {
					worstSim = r.sim
				}
			}
			// Actually we want the worst (lowest) in result.
			worstSim = float32(math.Inf(1))
			for _, r := range result {
				if r.sim < worstSim {
					worstSim = r.sim
				}
			}
		}

		if len(result) >= ef && c.sim < worstSim {
			break
		}

		// Expand neighbors.
		if lc < len(h.nodes[c.idx].links) {
			for _, nIdx := range h.nodes[c.idx].links[lc] {
				if nIdx == emptyLink || visited[nIdx] || h.nodes[nIdx].deleted {
					continue
				}
				visited[nIdx] = true
				sim := dotProduct(q, h.nodes[nIdx].vec)
				worstInResult := float32(math.Inf(1))
				if len(result) > 0 {
					for _, r := range result {
						if r.sim < worstInResult {
							worstInResult = r.sim
						}
					}
				}
				if len(result) < ef || sim > worstInResult {
					push(nIdx, sim)
					// Evict worst if over ef.
					if len(result) > ef {
						worstPos := 0
						for i, r := range result {
							if r.sim < result[worstPos].sim {
								worstPos = i
							}
						}
						result[worstPos] = result[len(result)-1]
						result = result[:len(result)-1]
					}
				}
			}
		}
	}

	// Sort result by sim descending.
	sortItems2(result)
	out := make([]uint32, len(result))
	for i, r := range result {
		out[i] = r.idx
	}
	return out
}

func (h *HNSWIndex) selectNeighbors(q []float32, candidates []uint32, m int) []uint32 {
	if len(candidates) <= m {
		return candidates
	}
	return candidates[:m]
}

// ── TopN ──────────────────────────────────────────────────────────────────────

func (h *HNSWIndex) TopN(queryRaw []float32, n int) ([]SearchResult, error) {
	if len(queryRaw) != h.dims {
		return nil, fmt.Errorf("zvec/hnsw: query dim %d != index dim %d", len(queryRaw), h.dims)
	}
	if h.entryIdx == -1 || n <= 0 {
		return nil, nil
	}

	q := make([]float32, h.dims)
	copy(q, queryRaw)
	normalizeVec(q)

	ef := maxInt(n, h.opts.efConstruct()/2)
	// Entry point may be deleted (e.g. WAL tombstone after a Save). Find a
	// live node to start from; if none, the index is empty.
	startIdx := h.entryIdx
	if h.nodes[startIdx].deleted {
		startIdx = -1
		for i, nd := range h.nodes {
			if !nd.deleted {
				startIdx = i
				break
			}
		}
		if startIdx == -1 {
			return nil, nil
		}
	}
	ep := []uint32{uint32(startIdx)}
	startLayer := h.entryLayer
	if startIdx != h.entryIdx {
		startLayer = h.nodes[startIdx].maxLayer
	}

	// Upper layers: ef=1 greedy walk.
	for lc := startLayer; lc > 0; lc-- {
		ep = h.searchLayer(q, ep, 1, lc)
	}

	// Layer 0: full search.
	candidates := h.searchLayer(q, ep, ef, 0)

	results := make([]SearchResult, 0, minInt(n, len(candidates)))
	for _, idx := range candidates {
		if len(results) >= n {
			break
		}
		node := h.nodes[idx]
		if node.deleted {
			continue
		}
		results = append(results, SearchResult{ID: node.id, Score: dotProduct(q, node.vec)})
	}
	return results, nil
}

// ── Mutations (WAL) ───────────────────────────────────────────────────────────

func (h *HNSWIndex) Upsert(path string, id uint64, rawVec []float32) error {
	if len(rawVec) != h.dims {
		return fmt.Errorf("zvec/hnsw: dim mismatch: got %d want %d", len(rawVec), h.dims)
	}
	// Soft-delete existing node if present.
	if idx, ok := h.idToIdx[id]; ok {
		h.nodes[idx].deleted = true
	}
	if err := h.Insert(id, rawVec); err != nil {
		return err
	}

	// Persist WAL.
	d := h.dims
	tailCount := d/lane - 1
	vec := make([]float32, d)
	copy(vec, rawVec)
	normalizeVec(vec)
	var tails []float32
	if tailCount > 0 {
		tails = make([]float32, tailCount)
		tailNorms(vec, tails)
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
	if err := binary.Write(f, binary.LittleEndian, vec); err != nil {
		return err
	}
	if tailCount > 0 {
		if err := binary.Write(f, binary.LittleEndian, tails); err != nil {
			return err
		}
	}
	h.walLatest[id] = walEntry{vec: vec, tails: tails}
	return nil
}

func (h *HNSWIndex) Delete(path string, id uint64) error {
	if idx, ok := h.idToIdx[id]; ok {
		h.nodes[idx].deleted = true
	}

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
	h.walLatest[id] = walEntry{}
	return nil
}

func (h *HNSWIndex) Compact(path string) error {
	fresh, err := NewHNSW(h.dims, h.opts)
	if err != nil {
		return err
	}
	for _, node := range h.nodes {
		if node.deleted {
			continue
		}
		if err := fresh.Insert(node.id, node.vec); err != nil {
			return err
		}
	}
	return fresh.Save(path)
}

// ── Serialization ─────────────────────────────────────────────────────────────

type hnswHeader struct {
	Magic      uint32
	Version    uint8
	LaneWidth  uint8
	Pad        uint16
	Dimensions uint32
	M          uint32
	M0         uint32
	Count      uint64
	EntryIdx   uint64
	EntryLayer uint32
	Pad2       uint32
}

// Save writes the index to path (no WAL — call Compact to produce a clean file).
func (h *HNSWIndex) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return h.writeTo(f)
}

func (h *HNSWIndex) writeTo(w io.Writer) error {
	entryIdx := uint64(math.MaxUint64)
	entryLayer := uint32(0)
	if h.entryIdx >= 0 {
		entryIdx = uint64(h.entryIdx)
		entryLayer = uint32(h.entryLayer)
	}

	hdr := hnswHeader{
		Magic:      hnswMagic,
		Version:    hnswVersion,
		LaneWidth:  lane,
		Dimensions: uint32(h.dims),
		M:          uint32(h.opts.m()),
		M0:         uint32(h.opts.m0()),
		Count:      uint64(len(h.nodes)),
		EntryIdx:   entryIdx,
		EntryLayer: entryLayer,
	}
	if err := binary.Write(w, binary.LittleEndian, &hdr); err != nil {
		return err
	}

	for _, node := range h.nodes {
		if err := binary.Write(w, binary.LittleEndian, node.id); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(node.maxLayer)); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, uint32(0)); err != nil { // pad
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, node.vec); err != nil {
			return err
		}
		if len(node.tails) > 0 {
			if err := binary.Write(w, binary.LittleEndian, node.tails); err != nil {
				return err
			}
		}
		for l := 0; l <= node.maxLayer; l++ {
			if err := binary.Write(w, binary.LittleEndian, node.links[l]); err != nil {
				return err
			}
		}
	}
	return nil
}

// LoadHNSW reads a .hnsw file.
func LoadHNSW(path string) (*HNSWIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readHNSW(f)
}

func readHNSW(r io.Reader) (*HNSWIndex, error) {
	var hdr hnswHeader
	if err := binary.Read(r, binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("zvec/hnsw: read header: %w", err)
	}
	if hdr.Magic != hnswMagic {
		return nil, fmt.Errorf("zvec/hnsw: bad magic %08x", hdr.Magic)
	}
	if hdr.Version != hnswVersion {
		return nil, fmt.Errorf("zvec/hnsw: unsupported version %d", hdr.Version)
	}

	d := int(hdr.Dimensions)
	n := int(hdr.Count)
	m := int(hdr.M)
	m0 := int(hdr.M0)
	tailCount := d/lane - 1

	opts := HNSWOptions{M: m, M0: m0}
	h := &HNSWIndex{
		dims:      d,
		opts:      opts,
		nodes:     make([]*hnswNode, 0, n),
		idToIdx:   make(map[uint64]int, n),
		entryIdx:  -1,
		walLatest: map[uint64]walEntry{},
	}

	if hdr.EntryIdx != math.MaxUint64 {
		h.entryIdx = int(hdr.EntryIdx)
		h.entryLayer = int(hdr.EntryLayer)
	}

	for i := 0; i < n; i++ {
		var id uint64
		var maxLayerU, pad uint32
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			return nil, fmt.Errorf("zvec/hnsw: node %d id: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &maxLayerU); err != nil {
			return nil, fmt.Errorf("zvec/hnsw: node %d max_layer: %w", i, err)
		}
		if err := binary.Read(r, binary.LittleEndian, &pad); err != nil {
			return nil, fmt.Errorf("zvec/hnsw: node %d pad: %w", i, err)
		}

		maxLayer := int(maxLayerU)
		vec := make([]float32, d)
		if err := binary.Read(r, binary.LittleEndian, vec); err != nil {
			return nil, fmt.Errorf("zvec/hnsw: node %d vec: %w", i, err)
		}
		tails := make([]float32, tailCount)
		if tailCount > 0 {
			if err := binary.Read(r, binary.LittleEndian, tails); err != nil {
				return nil, fmt.Errorf("zvec/hnsw: node %d tails: %w", i, err)
			}
		}

		links := make([][]uint32, maxLayer+1)
		for l := 0; l <= maxLayer; l++ {
			cap := m0
			if l > 0 {
				cap = m
			}
			links[l] = make([]uint32, cap)
			if err := binary.Read(r, binary.LittleEndian, links[l]); err != nil {
				return nil, fmt.Errorf("zvec/hnsw: node %d layer %d links: %w", i, l, err)
			}
		}

		node := &hnswNode{id: id, maxLayer: maxLayer, vec: vec, tails: tails, links: links}
		h.idToIdx[id] = len(h.nodes)
		h.nodes = append(h.nodes, node)
	}

	// Read WAL entries.
	for {
		var typ uint8
		if err := binary.Read(r, binary.LittleEndian, &typ); err != nil {
			break
		}
		var id uint64
		if err := binary.Read(r, binary.LittleEndian, &id); err != nil {
			break
		}
		switch typ {
		case walTombstone:
			h.walLatest[id] = walEntry{}
			if idx, ok := h.idToIdx[id]; ok {
				h.nodes[idx].deleted = true
			}
		case walUpsert:
			vec := make([]float32, d)
			if err := binary.Read(r, binary.LittleEndian, vec); err != nil {
				goto doneWAL
			}
			var tails []float32
			if tailCount > 0 {
				tails = make([]float32, tailCount)
				if err := binary.Read(r, binary.LittleEndian, tails); err != nil {
					goto doneWAL
				}
			}
			h.walLatest[id] = walEntry{vec: vec, tails: tails}
			// Soft-delete old node if present, insert new one.
			if idx, ok := h.idToIdx[id]; ok {
				h.nodes[idx].deleted = true
			}
			_ = h.Insert(id, vec) // vec is already normalized
		default:
			goto doneWAL
		}
	}
doneWAL:

	return h, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func dotProduct(a, b []float32) float32 {
	var s float32
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

// assignLayer returns a random max layer for a new node.
// Uses the HNSW paper's exponential distribution: floor(-ln(U) * mL)
// where mL = 1/ln(M).
func assignLayer(m int) int {
	mL := 1.0 / math.Log(float64(m))
	return int(math.Floor(-math.Log(rand.Float64()) * mL))
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
