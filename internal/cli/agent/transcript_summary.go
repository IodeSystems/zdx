package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// transcriptEvent is a minimal parse of one JSONL line for summarization.
type transcriptEvent struct {
	Type    string          `json:"type"`
	Title   string          `json:"title"`
	Message json.RawMessage `json:"message"`
}

// transcriptSummaryEntry is one rendered line in the summary.
type transcriptSummaryEntry struct {
	text    string
	verbose bool // false = always included, true = only in LOD head/tail
}

// SummarizeTranscript reads a Claude JSONL transcript and produces a
// condensed summary suitable as context for a fresh session continuing
// the same work. Uses level-of-detail: the first and last headEvents
// are rendered verbosely; middle events are compressed to one-liners.
//
// Also scans subagent transcripts in the subagent dir.
func SummarizeTranscript(transcriptPath, subagentDir string, headEvents, tailEvents int) string {
	if headEvents <= 0 {
		headEvents = 30
	}
	if tailEvents <= 0 {
		tailEvents = 40
	}

	entries := parseTranscriptEntries(transcriptPath)
	if len(entries) == 0 {
		return "(no transcript events found)"
	}

	var sb strings.Builder
	sb.WriteString("## Previous session transcript (stalled, auto-restarting)\n\n")

	total := len(entries)
	for i, e := range entries {
		inHead := i < headEvents
		inTail := i >= total-tailEvents
		if e.verbose && !inHead && !inTail {
			continue
		}
		sb.WriteString(e.text)
		sb.WriteByte('\n')

		// Insert elision marker at the boundary.
		if i == headEvents-1 && total > headEvents+tailEvents {
			skipped := total - headEvents - tailEvents
			sb.WriteString(fmt.Sprintf("\n... (%d events elided) ...\n\n", skipped))
		}
	}

	// Append subagent summaries if any.
	if subagentDir != "" {
		matches, _ := filepath.Glob(filepath.Join(subagentDir, "agent-*.jsonl"))
		for _, m := range matches {
			subEntries := parseTranscriptEntries(m)
			if len(subEntries) == 0 {
				continue
			}
			agentID := strings.TrimSuffix(filepath.Base(m), ".jsonl")
			sb.WriteString(fmt.Sprintf("\n### Subagent: %s\n", agentID))
			// Subagents get a tighter budget.
			subHead, subTail := 5, 10
			subTotal := len(subEntries)
			for i, e := range subEntries {
				inHead := i < subHead
				inTail := i >= subTotal-subTail
				if e.verbose && !inHead && !inTail {
					continue
				}
				sb.WriteString(e.text)
				sb.WriteByte('\n')
				if i == subHead-1 && subTotal > subHead+subTail {
					skipped := subTotal - subHead - subTail
					sb.WriteString(fmt.Sprintf("  ... (%d events elided) ...\n", skipped))
				}
			}
		}
	}

	return sb.String()
}

func parseTranscriptEntries(path string) []transcriptSummaryEntry {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var entries []transcriptSummaryEntry
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var evt transcriptEvent
		if json.Unmarshal(line, &evt) != nil {
			continue
		}
		switch evt.Type {
		case "ai-title":
			t := strings.TrimSpace(evt.Title)
			if t != "" {
				entries = append(entries, transcriptSummaryEntry{
					text: "● " + t, verbose: false,
				})
			}
		case "assistant":
			entries = append(entries, parseAssistantEntries(evt.Message)...)
		case "user":
			entries = append(entries, parseUserEntries(evt.Message)...)
		}
	}
	return entries
}

func parseAssistantEntries(msg json.RawMessage) []transcriptSummaryEntry {
	var m struct {
		Content []struct {
			Type     string          `json:"type"`
			Text     string          `json:"text"`
			Thinking string          `json:"thinking"`
			Name     string          `json:"name"`
			ID       string          `json:"id"`
			Input    json.RawMessage `json:"input"`
		} `json:"content"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return nil
	}
	var entries []transcriptSummaryEntry
	for _, c := range m.Content {
		switch c.Type {
		case "tool_use":
			summary := summarizeToolInput(c.Name, c.Input)
			entries = append(entries, transcriptSummaryEntry{
				text:    fmt.Sprintf("[%s] %s", c.Name, summary),
				verbose: false,
			})
		case "text":
			s := strings.TrimSpace(c.Text)
			if s == "" {
				continue
			}
			// Text blocks are verbose in middle — always shown in head/tail.
			entries = append(entries, transcriptSummaryEntry{
				text:    truncLines(s, 200),
				verbose: true,
			})
		case "thinking":
			s := strings.TrimSpace(c.Thinking)
			if s == "" {
				continue
			}
			entries = append(entries, transcriptSummaryEntry{
				text:    "⟡ " + flattenTrunc(s, 120),
				verbose: true,
			})
		}
	}
	return entries
}

func parseUserEntries(msg json.RawMessage) []transcriptSummaryEntry {
	var m struct {
		Content json.RawMessage `json:"content"`
	}
	if json.Unmarshal(msg, &m) != nil {
		return nil
	}
	raw := strings.TrimSpace(string(m.Content))
	if raw == "" {
		return nil
	}

	// String content = user prompt.
	if raw[0] == '"' {
		var s string
		if json.Unmarshal([]byte(raw), &s) != nil {
			return nil
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil
		}
		return []transcriptSummaryEntry{{
			text: "USER: " + truncLines(s, 300), verbose: false,
		}}
	}

	// Array content = tool results.
	if raw[0] != '[' {
		return nil
	}
	var arr []struct {
		Type      string          `json:"type"`
		ToolUseID string          `json:"tool_use_id"`
		IsError   bool            `json:"is_error"`
		Content   json.RawMessage `json:"content"`
	}
	if json.Unmarshal([]byte(raw), &arr) != nil {
		return nil
	}
	var entries []transcriptSummaryEntry
	for _, c := range arr {
		if c.Type != "tool_result" {
			continue
		}
		snippet := toolResultSnippet(c.Content, 150)
		prefix := "✓"
		if c.IsError {
			prefix = "✗"
		}
		entries = append(entries, transcriptSummaryEntry{
			text:    fmt.Sprintf("  %s %s", prefix, flattenTrunc(snippet, 150)),
			verbose: true,
		})
	}
	return entries
}

func summarizeToolInput(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	switch name {
	case "Bash":
		if s, ok := in["command"].(string); ok {
			return flattenTrunc(s, 120)
		}
	case "Read":
		if s, ok := in["file_path"].(string); ok {
			return s
		}
	case "Edit", "Write":
		if s, ok := in["file_path"].(string); ok {
			return s
		}
	case "Grep", "Glob":
		if s, ok := in["pattern"].(string); ok {
			path, _ := in["path"].(string)
			if path != "" {
				return s + " in " + path
			}
			return s
		}
	case "Agent":
		if s, ok := in["description"].(string); ok && s != "" {
			return s
		}
		if s, ok := in["subagent_type"].(string); ok {
			return s
		}
	case "WebFetch", "WebSearch":
		if s, ok := in["url"].(string); ok {
			return s
		}
		if s, ok := in["query"].(string); ok {
			return s
		}
	}
	return ""
}

// truncLines truncates multi-line text to at most n runes, preserving
// newlines within the budget.
func truncLines(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
