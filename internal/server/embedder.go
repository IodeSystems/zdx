package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/iodesystems/zdx-go/internal/llm"
	"github.com/iodesystems/zdx-go/internal/zvec"
)

// embedder holds the per-project zvec indexes and the active LLM client.
// It is safe for concurrent use; a RWMutex guards the client pointer.
type embedder struct {
	mu      sync.RWMutex
	client  *llm.Client // nil when no LLM config is set
	dataDir string      // directory where .zvec files are stored
	sink    timingSink
}

func newEmbedder(dataDir string, sink timingSink) *embedder {
	return &embedder{dataDir: dataDir, sink: sink}
}

// reload re-reads the LLM config from DB and replaces the active client.
func (e *embedder) Reload(cfg *llm.Config) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if cfg == nil {
		e.client = nil
		return
	}
	e.client = llm.New(*cfg)
}

// embed calls the LLM API to produce a float32 vector for text.
// Returns nil, nil when no LLM client is configured.
func (e *embedder) embed(ctx context.Context, text string) ([]float32, error) {
	e.mu.RLock()
	c := e.client
	e.mu.RUnlock()
	if c == nil {
		return nil, nil
	}
	start := time.Now()
	vec, err := c.Embed(ctx, text)
	e.sink.track(ctx, "llm:embed", start)
	return vec, err
}

// indexPath returns the .zvec file path for a given project ID and kind.
func (e *embedder) indexPath(projectID int32) string {
	return filepath.Join(e.dataDir, fmt.Sprintf("issues-%d.zvec", projectID))
}

func (e *embedder) questionIndexPath(projectID int32) string {
	return filepath.Join(e.dataDir, fmt.Sprintf("questions-%d.zvec", projectID))
}

// upsertIssue embeds issueText and upserts it into the project's zvec index.
// Silently no-ops when no LLM client is configured.
func (e *embedder) UpsertIssue(ctx context.Context, projectID int32, issueID string, issueText string) {
	vec, err := e.embed(ctx, issueText)
	if err != nil {
		log.Printf("embedder: embed issue %s: %v", issueID, err)
		return
	}
	if vec == nil {
		return
	}

	id := issueNumericID(issueID)
	path := e.indexPath(projectID)

	start := time.Now()
	idx, err := e.loadOrCreate(path, len(vec))
	if err != nil {
		log.Printf("embedder: open index %s: %v", path, err)
		return
	}
	if err := idx.Upsert(path, id, vec); err != nil {
		log.Printf("embedder: upsert %s: %v", issueID, err)
	}
	e.sink.track(ctx, "zvec:upsert-issue", start)
}

// topN returns up to n issues most similar to queryText from the project index.
func (e *embedder) TopN(ctx context.Context, projectID int32, queryText string, n int) ([]zvec.SearchResult, error) {
	vec, err := e.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if vec == nil {
		return nil, nil // no LLM configured
	}

	path := e.indexPath(projectID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil // index not built yet
	}

	start := time.Now()
	idx, err := zvec.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open index: %w", err)
	}
	results, err := idx.TopN(vec, n)
	e.sink.track(ctx, "zvec:search-issues", start)
	return results, err
}

// loadOrCreate loads an existing index or creates a new one if missing.
// dims is used only when creating; existing index dims must match.
func (e *embedder) loadOrCreate(path string, dims int) (zvec.Index, error) {
	if _, err := os.Stat(path); err == nil {
		return zvec.Open(path)
	}
	// Ensure directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	b, err := zvec.NewBuilder(roundUp8(dims))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := b.Serialize(&buf); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return nil, err
	}
	return zvec.LoadBytes(buf.Bytes())
}

// roundUp8 rounds dims up to the nearest multiple of 8 (zvec requirement).
func roundUp8(d int) int {
	if d%8 == 0 {
		return d
	}
	return d + (8 - d%8)
}

// upsertQuestion embeds questionText and upserts it into the project's question zvec index.
func (e *embedder) UpsertQuestion(ctx context.Context, projectID int32, questionID int32, questionText string) {
	vec, err := e.embed(ctx, questionText)
	if err != nil {
		log.Printf("embedder: embed question %d: %v", questionID, err)
		return
	}
	if vec == nil {
		return
	}

	path := e.questionIndexPath(projectID)
	start := time.Now()
	idx, err := e.loadOrCreate(path, len(vec))
	if err != nil {
		log.Printf("embedder: open question index %s: %v", path, err)
		return
	}
	if err := idx.Upsert(path, uint64(questionID), vec); err != nil { //nolint:gosec
		log.Printf("embedder: upsert question %d: %v", questionID, err)
	}
	e.sink.track(ctx, "zvec:upsert-question", start)
}

func (e *embedder) Complete(ctx context.Context, messages []llm.ChatMessage) (string, error) {
	e.mu.RLock()
	c := e.client
	e.mu.RUnlock()
	if c == nil {
		return "", fmt.Errorf("no LLM client configured")
	}
	return c.Complete(ctx, messages)
}

func (e *embedder) StreamComplete(ctx context.Context, messages []llm.ChatMessage, onDelta func(string)) (string, error) {
	e.mu.RLock()
	c := e.client
	e.mu.RUnlock()
	if c == nil {
		return "", fmt.Errorf("no LLM client configured")
	}
	return c.StreamChatCompletion(ctx, messages, onDelta)
}

// topNQuestions returns up to n questions most similar to queryText.
func (e *embedder) TopNQuestions(ctx context.Context, projectID int32, queryText string, n int) ([]zvec.SearchResult, error) {
	vec, err := e.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if vec == nil {
		return nil, nil
	}

	path := e.questionIndexPath(projectID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	start := time.Now()
	idx, err := zvec.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open question index: %w", err)
	}
	results, err := idx.TopN(vec, n)
	e.sink.track(ctx, "zvec:search-questions", start)
	return results, err
}

func (e *embedder) taskIndexPath(projectID int32) string {
	return filepath.Join(e.dataDir, fmt.Sprintf("tasks-%d.zvec", projectID))
}

func (e *embedder) UpsertTask(ctx context.Context, projectID int32, taskID string, taskText string) {
	vec, err := e.embed(ctx, taskText)
	if err != nil {
		log.Printf("embedder: embed task %s: %v", taskID, err)
		return
	}
	if vec == nil {
		return
	}

	id := taskNumericID(taskID)
	path := e.taskIndexPath(projectID)

	start := time.Now()
	idx, err := e.loadOrCreate(path, len(vec))
	if err != nil {
		log.Printf("embedder: open task index %s: %v", path, err)
		return
	}
	if err := idx.Upsert(path, id, vec); err != nil {
		log.Printf("embedder: upsert task %s: %v", taskID, err)
	}
	e.sink.track(ctx, "zvec:upsert-task", start)
}

func (e *embedder) TopNTasks(ctx context.Context, projectID int32, queryText string, n int) ([]zvec.SearchResult, error) {
	vec, err := e.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if vec == nil {
		return nil, nil
	}

	path := e.taskIndexPath(projectID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	start := time.Now()
	idx, err := zvec.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open task index: %w", err)
	}
	results, err := idx.TopN(vec, n)
	e.sink.track(ctx, "zvec:search-tasks", start)
	return results, err
}

func (e *embedder) patternIndexPath(projectID int32) string {
	return filepath.Join(e.dataDir, fmt.Sprintf("patterns-%d.zvec", projectID))
}

func (e *embedder) UpsertPattern(ctx context.Context, projectID int32, patternID int32, patternText string) {
	vec, err := e.embed(ctx, patternText)
	if err != nil {
		log.Printf("embedder: embed pattern %d: %v", patternID, err)
		return
	}
	if vec == nil {
		return
	}

	path := e.patternIndexPath(projectID)
	start := time.Now()
	idx, err := e.loadOrCreate(path, len(vec))
	if err != nil {
		log.Printf("embedder: open pattern index %s: %v", path, err)
		return
	}
	if err := idx.Upsert(path, uint64(patternID), vec); err != nil { //nolint:gosec
		log.Printf("embedder: upsert pattern %d: %v", patternID, err)
	}
	e.sink.track(ctx, "zvec:upsert-pattern", start)
}

func (e *embedder) TopNPatterns(ctx context.Context, projectID int32, queryText string, n int) ([]zvec.SearchResult, error) {
	vec, err := e.embed(ctx, queryText)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}
	if vec == nil {
		return nil, nil
	}

	path := e.patternIndexPath(projectID)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}

	start := time.Now()
	idx, err := zvec.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open pattern index: %w", err)
	}
	results, err := idx.TopN(vec, n)
	e.sink.track(ctx, "zvec:search-patterns", start)
	return results, err
}

// issueNumericID converts "IS-42" → 42.
func issueNumericID(id string) uint64 {
	s := strings.TrimPrefix(id, "IS-")
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}

func taskNumericID(id string) uint64 {
	s := strings.TrimPrefix(id, "TK-")
	n, _ := strconv.ParseUint(s, 10, 64)
	return n
}
