package tracelog

import (
	"bufio"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

type captured struct {
	level   string
	message string
	source  string
	tags    zdxclient.Tags
}

type fakeSink struct {
	mu   sync.Mutex
	rows []captured
}

func (f *fakeSink) RecordLogWithSource(level, message, source string, tags zdxclient.Tags) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, captured{level, message, source, tags})
}

func TestLogger_EmitsToFileAndSinkWithTraceAndCallSite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "agent.log")
	sink := &fakeSink{}

	l, err := New(Options{
		BaseTags: map[string]string{
			"agent":      "opencode",
			"session_id": "sess-1",
		},
		FilePath: path,
		Sink:     sink,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if l.TraceID() == "" {
		t.Fatal("expected trace id to be generated")
	}

	l.Info("session.start", "issue", "TK-1", "model", "claude-sonnet-4-6")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if len(sink.rows) != 1 {
		t.Fatalf("sink rows = %d; want 1", len(sink.rows))
	}
	row := sink.rows[0]
	if row.level != "info" || row.message != "session.start" {
		t.Fatalf("bad row: %+v", row)
	}
	if row.tags["trace_id"] != l.TraceID() {
		t.Errorf("trace_id missing on sink emit: %v", row.tags)
	}
	if row.tags["agent"] != "opencode" || row.tags["session_id"] != "sess-1" {
		t.Errorf("base tags missing: %v", row.tags)
	}
	if row.tags["issue"] != "TK-1" || row.tags["model"] != "claude-sonnet-4-6" {
		t.Errorf("kv tags missing: %v", row.tags)
	}
	if !strings.HasPrefix(row.tags["code.file"], "tracelog_test.go") {
		t.Errorf("code.file = %q; want tracelog_test.go", row.tags["code.file"])
	}
	if row.tags["code.line"] == "" || row.tags["code.func"] == "" {
		t.Errorf("code.line / code.func missing: %v", row.tags)
	}
	if !strings.Contains(row.source, "tracelog_test.go:") {
		t.Errorf("source = %q; want contains tracelog_test.go:", row.source)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("expected at least one line in file")
	}
	var rec map[string]any
	if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if rec["msg"] != "session.start" {
		t.Errorf("file msg = %v", rec["msg"])
	}
	tags, _ := rec["tags"].(map[string]any)
	if tags["trace_id"] != l.TraceID() {
		t.Errorf("file trace_id missing: %v", tags)
	}
}

func TestLogger_With_MergesAndOverrides(t *testing.T) {
	sink := &fakeSink{}
	l, err := New(Options{
		BaseTags: map[string]string{"agent": "opencode", "phase": "init"},
		Sink:     sink,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	child := l.With(map[string]string{"phase": "running", "container_id": "abc"})
	child.Warn("tool.failed")

	if len(sink.rows) != 1 {
		t.Fatalf("sink rows = %d", len(sink.rows))
	}
	tags := sink.rows[0].tags
	if tags["phase"] != "running" {
		t.Errorf("child should override phase, got %q", tags["phase"])
	}
	if tags["agent"] != "opencode" {
		t.Errorf("agent base tag lost: %q", tags["agent"])
	}
	if tags["container_id"] != "abc" {
		t.Errorf("new child tag missing")
	}
	if tags["trace_id"] != l.TraceID() {
		t.Errorf("trace_id should be inherited")
	}
}

func TestLogger_FilteredURL(t *testing.T) {
	l, err := New(Options{
		TraceID: "trace-fixed-1",
		Sink:    &fakeSink{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()

	got := l.FilteredURL("https://zdx.example.com/", "my-proj")
	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if u.Path != "/project/my-proj/logs" {
		t.Errorf("path = %q", u.Path)
	}
	tagFilter := u.Query().Get("tag_filter")
	if !strings.Contains(tagFilter, "trace-fixed-1") {
		t.Errorf("tag_filter missing trace id: %q", tagFilter)
	}

	if l.FilteredURL("", "x") != "" || l.FilteredURL("y", "") != "" {
		t.Error("FilteredURL should return empty when remote or slug is empty")
	}
}

func TestLogger_NoSinkOrFile_DoesNotPanic(t *testing.T) {
	l, err := New(Options{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer l.Close()
	l.Info("noop", "k", "v")
}
