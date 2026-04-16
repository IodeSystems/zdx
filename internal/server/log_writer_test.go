package server

import (
	"bytes"
	"log"
	"sync"
	"testing"
)

type capturedLine struct {
	level, message string
}

type captureForwarder struct {
	mu    sync.Mutex
	lines []capturedLine
}

func (c *captureForwarder) forward(level, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, capturedLine{level, message})
}

func (c *captureForwarder) snapshot() []capturedLine {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]capturedLine, len(c.lines))
	copy(out, c.lines)
	return out
}

func TestLogWriter_StripsStdPrefixAndForwards(t *testing.T) {
	var stderr bytes.Buffer
	cap := &captureForwarder{}
	w := &LogWriter{Primary: &stderr, Forward: cap.forward}

	logger := log.New(w, "", log.LstdFlags)
	logger.Print("hello world")

	got := cap.snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 forwarded line, got %d (%v)", len(got), got)
	}
	if got[0].level != "info" {
		t.Errorf("want level=info, got %q", got[0].level)
	}
	if got[0].message != "hello world" {
		t.Errorf("want message=%q, got %q", "hello world", got[0].message)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("hello world")) {
		t.Errorf("primary did not receive the log line: %q", stderr.String())
	}
}

func TestLogWriter_SuppressesFeedbackLoops(t *testing.T) {
	var stderr bytes.Buffer
	cap := &captureForwarder{}
	w := &LogWriter{
		Primary:  &stderr,
		Forward:  cap.forward,
		Suppress: []string{"zdxclient", "InsertLogEvent"},
	}

	logger := log.New(w, "", log.LstdFlags)
	logger.Print("zdxclient: 5 events dropped: timeout")
	logger.Print("ingest: InsertLogEvent: db error")
	logger.Print("legitimate message")

	got := cap.snapshot()
	if len(got) != 1 {
		t.Fatalf("want 1 forwarded line, got %d (%v)", len(got), got)
	}
	if got[0].message != "legitimate message" {
		t.Errorf("unexpected forwarded message: %q", got[0].message)
	}
	// Both suppressed lines must still hit stderr.
	if !bytes.Contains(stderr.Bytes(), []byte("zdxclient: 5 events dropped")) {
		t.Errorf("suppressed line missing from stderr: %q", stderr.String())
	}
	if !bytes.Contains(stderr.Bytes(), []byte("InsertLogEvent")) {
		t.Errorf("suppressed line missing from stderr: %q", stderr.String())
	}
}

func TestLogWriter_BuffersPartialLines(t *testing.T) {
	var stderr bytes.Buffer
	cap := &captureForwarder{}
	w := &LogWriter{Primary: &stderr, Forward: cap.forward}

	if _, err := w.Write([]byte("first ")); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("half")); err != nil {
		t.Fatal(err)
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Fatalf("partial line forwarded prematurely: %v", got)
	}
	if _, err := w.Write([]byte(" complete\nsecond\n")); err != nil {
		t.Fatal(err)
	}
	got := cap.snapshot()
	if len(got) != 2 {
		t.Fatalf("want 2 lines, got %d (%v)", len(got), got)
	}
	if got[0].message != "first half complete" {
		t.Errorf("line[0] = %q", got[0].message)
	}
	if got[1].message != "second" {
		t.Errorf("line[1] = %q", got[1].message)
	}
}

func TestLogWriter_NilForwardJustWritesToPrimary(t *testing.T) {
	var stderr bytes.Buffer
	w := &LogWriter{Primary: &stderr}
	if _, err := w.Write([]byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	if stderr.String() != "hello\n" {
		t.Errorf("primary mismatch: %q", stderr.String())
	}
}

func TestLogWriter_DropsEmptyLines(t *testing.T) {
	var stderr bytes.Buffer
	cap := &captureForwarder{}
	w := &LogWriter{Primary: &stderr, Forward: cap.forward}
	if _, err := w.Write([]byte("\n\n   \n")); err != nil {
		t.Fatal(err)
	}
	if got := cap.snapshot(); len(got) != 0 {
		t.Fatalf("empty lines forwarded: %v", got)
	}
}
