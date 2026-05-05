// Package tracelog wires the agent's structured-log emission to (1) a local
// JSONL file under .zdx/logs and (2) the zdx server's /api/ingest/logs sink,
// stamping every event with a trace_id and the call site (code.file/code.line/
// code.func). It returns a tag-filtered UI URL so an operator running
// `dx agent opencode` can click through to a live timeline of the run.
//
// Design notes:
//   - Loggers are cheap, immutable values. With() returns a derived logger
//     whose tags merge over the parent's; the underlying sinks are shared.
//   - Emission is non-blocking by design — the file write is buffered, the
//     network ship is fire-and-forget through zdxclient. A failed log line
//     never breaks the agent.
//   - Token/endpoint are dependency-injected. Callers that can't resolve an
//     ingest token still get a working file-only logger.
package tracelog

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iodesystems/zdx-go/pkg/zdxclient"
)

// Level is the log severity stamped on each event. Mirrors the zdx_log_events
// .level column values.
type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
)

// Sink is the network shipper. *zdxclient.Client satisfies this; tests can
// pass a fake. nil disables network shipping.
type Sink interface {
	RecordLogWithSource(level, message, source string, tags zdxclient.Tags)
}

// Options configures NewLogger.
type Options struct {
	// TraceID, if empty, is generated as a uuid v4. Reuse across a session.
	TraceID string

	// BaseTags are stamped on every emit. Common keys: agent, session_id,
	// issue_id, slug, branch, worktree, container_id, alias, model.
	BaseTags map[string]string

	// FilePath is the local JSONL sink (typically .zdx/logs/<sid>.log).
	// Empty disables file output.
	FilePath string

	// Sink is the network shipper. nil disables network output.
	Sink Sink

	// Component is reserved for future use (multi-component sessions); for
	// now it's stamped onto each event's tags as "component".
	Component string
}

// Logger emits structured events to a local JSONL file and the zdx ingest
// endpoint, both tagged with trace_id and the call site.
type Logger struct {
	traceID   string
	tags      map[string]string
	component string

	sink Sink

	fileMu *sync.Mutex
	file   *os.File
}

// New constructs a Logger. If FilePath is set, the file is opened in append
// mode (creating parent dirs if needed). Network shipping is enabled iff Sink
// is non-nil.
func New(opts Options) (*Logger, error) {
	tid := opts.TraceID
	if tid == "" {
		tid = uuid.NewString()
	}
	tags := map[string]string{}
	for k, v := range opts.BaseTags {
		tags[k] = v
	}
	tags["trace_id"] = tid

	l := &Logger{
		traceID:   tid,
		tags:      tags,
		component: opts.Component,
		sink:      opts.Sink,
		fileMu:    &sync.Mutex{},
	}

	if opts.FilePath != "" {
		if err := os.MkdirAll(filepath.Dir(opts.FilePath), 0o755); err != nil {
			return nil, fmt.Errorf("mkdir log dir: %w", err)
		}
		f, err := os.OpenFile(opts.FilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log file: %w", err)
		}
		l.file = f
	}

	return l, nil
}

// TraceID returns the immutable trace id stamped on every event.
func (l *Logger) TraceID() string { return l.traceID }

// With returns a derived logger whose tags merge over the parent's. Both
// loggers share the same file handle and sink — closing one does not affect
// the other beyond the file (callers should close only the root).
func (l *Logger) With(extra map[string]string) *Logger {
	merged := make(map[string]string, len(l.tags)+len(extra))
	for k, v := range l.tags {
		merged[k] = v
	}
	for k, v := range extra {
		merged[k] = v
	}
	return &Logger{
		traceID:   l.traceID,
		tags:      merged,
		component: l.component,
		sink:      l.sink,
		fileMu:    l.fileMu,
		file:      l.file,
	}
}

// Info emits an info-level event. kv is a flat list of alternating string
// keys and values (any type — formatted via %v). An odd-length list will
// log the final key with an empty value.
func (l *Logger) Info(msg string, kv ...any) { l.emit(LevelInfo, msg, 1, kv) }

// Warn emits a warn-level event.
func (l *Logger) Warn(msg string, kv ...any) { l.emit(LevelWarn, msg, 1, kv) }

// Error emits an error-level event.
func (l *Logger) Error(msg string, kv ...any) { l.emit(LevelError, msg, 1, kv) }

// Debug emits a debug-level event.
func (l *Logger) Debug(msg string, kv ...any) { l.emit(LevelDebug, msg, 1, kv) }

func (l *Logger) emit(level Level, msg string, callerSkip int, kv []any) {
	tags := make(map[string]string, len(l.tags)+len(kv)/2+3)
	for k, v := range l.tags {
		tags[k] = v
	}
	for i := 0; i < len(kv); i += 2 {
		key, _ := kv[i].(string)
		if key == "" {
			continue
		}
		var val string
		if i+1 < len(kv) {
			val = fmt.Sprintf("%v", kv[i+1])
		}
		tags[key] = val
	}

	source, codeFile, codeLine, codeFunc := captureCaller(callerSkip + 1)
	if codeFile != "" {
		tags["code.file"] = codeFile
		tags["code.line"] = fmt.Sprintf("%d", codeLine)
		tags["code.func"] = codeFunc
	}

	if l.file != nil {
		l.writeFile(level, msg, source, tags)
	}
	if l.sink != nil {
		l.sink.RecordLogWithSource(string(level), msg, source, zdxclient.Tags(tags))
	}
}

func (l *Logger) writeFile(level Level, msg, source string, tags map[string]string) {
	rec := map[string]any{
		"ts":     time.Now().UTC().Format(time.RFC3339Nano),
		"level":  string(level),
		"msg":    msg,
		"source": source,
		"tags":   tags,
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return
	}
	l.fileMu.Lock()
	defer l.fileMu.Unlock()
	_, _ = l.file.Write(append(line, '\n'))
}

// Close flushes and closes the file sink. The network sink is owned by the
// caller — Close does not touch it.
func (l *Logger) Close() error {
	if l.file == nil {
		return nil
	}
	l.fileMu.Lock()
	defer l.fileMu.Unlock()
	err := l.file.Close()
	l.file = nil
	return err
}

// FilteredURL builds a deep link to the zdx UI logs view, pre-filtered to
// this logger's trace_id. remoteBase is the server origin (e.g. "https://
// zdx.example.com"). slug is the project slug.
func (l *Logger) FilteredURL(remoteBase, slug string) string {
	if remoteBase == "" || slug == "" {
		return ""
	}
	filter, _ := json.Marshal(map[string]string{"trace_id": l.traceID})
	return fmt.Sprintf("%s/project/%s/logs?tag_filter=%s",
		strings.TrimRight(remoteBase, "/"),
		url.PathEscape(slug),
		url.QueryEscape(string(filter)))
}

// captureCaller returns (source, file, line, funcName) for the caller `skip`
// frames up the stack. source is "<file>:<line>" suitable for the
// zdx_log_events.source column; file is the basename only.
func captureCaller(skip int) (source, file string, line int, funcName string) {
	pc, fullFile, ln, ok := runtime.Caller(skip + 1)
	if !ok {
		return "", "", 0, ""
	}
	base := filepath.Base(fullFile)
	fn := runtime.FuncForPC(pc)
	if fn != nil {
		funcName = trimFuncName(fn.Name())
	}
	return fmt.Sprintf("%s:%d", base, ln), base, ln, funcName
}

// trimFuncName drops the package import path prefix, leaving the last two
// path components (so "github.com/iodesystems/zdx-go/internal/cli/agent.runOpenCodeSession"
// becomes "agent.runOpenCodeSession").
func trimFuncName(name string) string {
	if idx := strings.LastIndex(name, "/"); idx >= 0 {
		return name[idx+1:]
	}
	return name
}
