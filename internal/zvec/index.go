package zvec

import (
	"encoding/binary"
	"fmt"
	"os"
)

// Index is the common interface for all zvec index implementations.
// Both the flat (exact) and HNSW (approximate) backends satisfy it.
//
// Mutations (Upsert/Delete) are append-only to the file on disk;
// Compact rewrites a clean file. The in-memory state is updated immediately
// so the index remains searchable without a reload.
type Index interface {
	// Dims returns the vector dimensionality.
	Dims() int

	// Count returns the number of live vectors.
	Count() int

	// TopN returns up to n vectors most similar to queryRaw (cosine similarity),
	// sorted descending by score.
	TopN(queryRaw []float32, n int) ([]SearchResult, error)

	// Upsert appends a WAL upsert entry for id to path and updates in-memory state.
	// rawVec is the un-normalized original-space vector.
	Upsert(path string, id uint64, rawVec []float32) error

	// Delete appends a WAL tombstone for id to path and updates in-memory state.
	Delete(path string, id uint64) error

	// Compact rewrites path as a clean index with no WAL overhead.
	Compact(path string) error
}

// Open loads a .zvec or .hnsw file and returns the appropriate Index.
// The file type is detected from the magic bytes in the header.
func Open(path string) (Index, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var magic uint32
	if err := binary.Read(f, binary.LittleEndian, &magic); err != nil {
		return nil, fmt.Errorf("zvec: read magic: %w", err)
	}

	switch magic {
	case hnswMagic:
		return LoadHNSW(path)
	default: // zvec magic or unknown — let Load handle the error
		return Load(path)
	}
}

// compile-time assertions.
var _ Index = (*FlatIndex)(nil)
var _ Index = (*HNSWIndex)(nil)
