package devtools

import (
	"encoding/json"
	"fmt"
	"github.com/iodesystems/zdx-go/internal/cli"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func ClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "claude", Short: "Claude session management"}
	cmd.AddCommand(claudeSummarizeCmd())
	return cmd
}

type claudeSession struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	Title     string `json:"title"`
	Header    string `json:"header"`
	Summary   string `json:"summary"`
	Status    string `json:"status"`
}

type claudeEvent struct {
	Seq       int32           `json:"seq"`
	EventType string          `json:"event_type"`
	EventJSON json.RawMessage `json:"event_json"`
}

func claudeSummarizeCmd() *cobra.Command {
	var sessionID int64
	var all bool
	cmd := &cobra.Command{
		Use:   "summarize",
		Short: "Generate header, summary, and status for sessions via claude -p",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()

			if sessionID > 0 {
				return summarizeSession(c, slug, sessionID)
			}

			var resp struct {
				Sessions []claudeSession `json:"sessions"`
			}
			params := url.Values{"slug": {slug}, "limit": {"100"}}
			if err := c.Get("/api/dx/claude/sessions", params, &resp); err != nil {
				return err
			}

			for _, s := range resp.Sessions {
				if !all && s.Summary != "" {
					continue
				}
				fmt.Fprintf(os.Stderr, "summarizing session %d (%s)...\n", s.ID, s.Title)
				if err := summarizeSession(c, slug, s.ID); err != nil {
					fmt.Fprintf(os.Stderr, "  error: %v\n", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().Int64Var(&sessionID, "session", 0, "specific session ID to summarize")
	cmd.Flags().BoolVar(&all, "all", false, "re-summarize sessions that already have summaries")
	return cmd
}

func summarizeSession(c *cli.Client, slug string, sessionID int64) error {
	params := url.Values{"slug": {slug}, "limit": {"500"}}
	var resp struct {
		Events []claudeEvent `json:"events"`
		Total  int64         `json:"total"`
	}
	if err := c.Get("/api/dx/claude/sessions/"+strconv.FormatInt(sessionID, 10)+"/events", params, &resp); err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}

	var transcript strings.Builder
	for _, ev := range resp.Events {
		var parsed map[string]any
		if json.Unmarshal(ev.EventJSON, &parsed) != nil {
			continue
		}
		msg, _ := parsed["message"].(map[string]any)
		if msg == nil {
			continue
		}
		role, _ := msg["role"].(string)
		content := extractTextContent(msg["content"])
		if content == "" {
			continue
		}
		if role == "user" || role == "assistant" {
			transcript.WriteString("[" + role + "] ")
			if len(content) > 2000 {
				content = content[:2000] + "..."
			}
			transcript.WriteString(content)
			transcript.WriteString("\n\n")
		}
	}

	if transcript.Len() == 0 {
		return fmt.Errorf("no transcript content")
	}

	prompt := `Analyze this Claude Code session transcript and return JSON with exactly three fields:
- "header": one sentence describing the session goal (what the user set out to do)
- "summary": 2-4 sentences describing what happened, what was accomplished, and any notable outcomes
- "status": one of "ok", "churn", or "errored" based on:
  - "ok": session completed its goal without significant issues
  - "churn": session had repeated edits to the same file, went in circles, or retried failing approaches
  - "errored": session ended with unresolved errors or tool failures

Return ONLY valid JSON, no markdown fences, no explanation.

Transcript:
` + transcript.String()

	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}
	out, err := exec.Command(claudePath, "-p", prompt, "--output-format", "text").CombinedOutput()
	if err != nil {
		return fmt.Errorf("claude -p failed: %w\n%s", err, out)
	}

	result := strings.TrimSpace(string(out))
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimPrefix(result, "```")
	result = strings.TrimSuffix(result, "```")
	result = strings.TrimSpace(result)

	var summary struct {
		Header  string `json:"header"`
		Summary string `json:"summary"`
		Status  string `json:"status"`
	}
	if err := json.Unmarshal([]byte(result), &summary); err != nil {
		return fmt.Errorf("parse claude response: %w\nraw: %s", err, result)
	}

	if summary.Status != "ok" && summary.Status != "churn" && summary.Status != "errored" {
		summary.Status = "ok"
	}

	patchURL := fmt.Sprintf("/api/dx/claude/sessions/%d/summary?slug=%s", sessionID, url.QueryEscape(slug))
	var patchResp struct {
		OK bool `json:"ok"`
	}
	if err := c.Patch(patchURL, summary, &patchResp); err != nil {
		return fmt.Errorf("patch summary: %w", err)
	}

	fmt.Printf("  header: %s\n  status: %s\n  summary: %s\n", summary.Header, summary.Status, summary.Summary)
	return nil
}

func extractTextContent(content any) string {
	if s, ok := content.(string); ok {
		return s
	}
	blocks, ok := content.([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, b := range blocks {
		block, ok := b.(map[string]any)
		if !ok {
			continue
		}
		if block["type"] == "text" {
			if t, ok := block["text"].(string); ok {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
}
