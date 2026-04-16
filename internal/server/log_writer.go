package server

import (
	"bytes"
	"io"
	"regexp"
	"strings"
	"sync"
)

// stdLogPrefix matches the default "2006/01/02 15:04:05[.000000] " prefix that
// the Go log package emits with LstdFlags. We strip it before forwarding so
// the dashboard message column doesn't carry a duplicate timestamp.
var stdLogPrefix = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(\.\d+)?\s+`)

// LogForwardFunc receives one log line at a time after prefix-stripping and
// suppression. It must be non-blocking — the writer holds its mutex while
// calling it. zdxclient.RecordLog is non-blocking by design.
type LogForwardFunc func(level, message string)

// LogWriter is an io.Writer adapter for log.SetOutput. It writes every byte
// through to a primary sink (typically os.Stderr) and, in parallel, splits
// the stream into newline-terminated lines and forwards each non-suppressed
// line via Forward. Partial writes are buffered until a newline arrives.
//
// Suppress holds substrings that disqualify a line from being forwarded —
// used to break feedback loops (e.g. log lines emitted by the forwarder's
// own failure path).
type LogWriter struct {
	Primary  io.Writer
	Forward  LogForwardFunc
	Suppress []string

	mu  sync.Mutex
	buf []byte
}

func (w *LogWriter) Write(p []byte) (int, error) {
	n, err := w.Primary.Write(p)
	if w.Forward == nil {
		return n, err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := w.buf[:i]
		w.buf = w.buf[i+1:]
		w.handleLine(line)
	}
	return n, err
}

func (w *LogWriter) handleLine(line []byte) {
	s := strings.TrimRight(string(line), "\r")
	s = stdLogPrefix.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	if s == "" {
		return
	}
	for _, sub := range w.Suppress {
		if strings.Contains(s, sub) {
			return
		}
	}
	w.Forward("info", s)
}
