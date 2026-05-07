package mcpcmd

import (
	"fmt"
	"strings"
)

// truncateLongLines walks text line-by-line; lines longer than maxLineChars
// are cut and annotated with a marker recording the original length. Returns
// the rewritten text plus counts (so the caller can surface truncation in
// tool result metadata). Pass maxLineChars<=0 to disable.
//
// Why this exists: a single ~400KB line in a generated file (e.g.
// internal/dxclient/openapi.json is 415KB on one line) will blow downstream
// chat-template buffers if it ever gets matched by grep or read by read_file
// without offset/limit. Per-line truncation keeps individual lines bounded
// regardless of how the file is shaped.
func truncateLongLines(text string, maxLineChars int) (out string, truncatedLines int, truncatedBytes int) {
	if maxLineChars <= 0 || text == "" {
		return text, 0, 0
	}
	var b strings.Builder
	b.Grow(len(text))
	lineNo := 0
	start := 0
	for i := 0; i <= len(text); i++ {
		if i < len(text) && text[i] != '\n' {
			continue
		}
		line := text[start:i]
		lineNo++
		if len(line) > maxLineChars {
			truncatedLines++
			truncatedBytes += len(line) - maxLineChars
			b.WriteString(line[:maxLineChars])
			fmt.Fprintf(&b, "…[line %d truncated; original=%dch]", lineNo, len(line))
		} else {
			b.WriteString(line)
		}
		if i < len(text) {
			b.WriteByte('\n')
		}
		start = i + 1
	}
	return b.String(), truncatedLines, truncatedBytes
}
