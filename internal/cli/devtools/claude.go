package devtools

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

func ClaudeCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "claude", Short: "Claude session management"}
	cmd.AddCommand(claudeSummarizeCmd())
	return cmd
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
				return summarizeSession(cmd, c, slug, sessionID)
			}

			limit := int32(100)
			resp, err := c.ListClaudeSessionsWithResponse(cmd.Context(), &dxclient.ListClaudeSessionsParams{
				Slug:  slug,
				Limit: &limit,
			})
			if err != nil {
				return err
			}
			if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
				return err
			}
			if resp.JSON200 == nil || resp.JSON200.Sessions == nil {
				return nil
			}

			for _, s := range *resp.JSON200.Sessions {
				if !all && s.Summary != "" {
					continue
				}
				fmt.Fprintf(os.Stderr, "summarizing session %d (%s)...\n", s.Id, s.Title)
				if err := summarizeSession(cmd, c, slug, s.Id); err != nil {
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

func summarizeSession(cmd *cobra.Command, c *cli.Client, slug string, sessionID int64) error {
	limit := int32(500)
	resp, err := c.GetClaudeSessionEventsWithResponse(cmd.Context(), sessionID, &dxclient.GetClaudeSessionEventsParams{
		Slug:  slug,
		Limit: &limit,
	})
	if err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return fmt.Errorf("fetch events: %w", err)
	}
	if resp.JSON200 == nil || resp.JSON200.Events == nil {
		return fmt.Errorf("no events")
	}

	var transcript strings.Builder
	for _, ev := range *resp.JSON200.Events {
		parsed, ok := ev.EventJson.(map[string]any)
		if !ok {
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

	var summary dxclient.UpdateClaudeSessionSummaryRequest
	if err := json.Unmarshal([]byte(result), &summary); err != nil {
		return fmt.Errorf("parse claude response: %w\nraw: %s", err, result)
	}

	if summary.Status != "ok" && summary.Status != "churn" && summary.Status != "errored" {
		summary.Status = "ok"
	}

	patchResp, err := c.UpdateClaudeSessionSummaryWithResponse(cmd.Context(), sessionID,
		&dxclient.UpdateClaudeSessionSummaryParams{Slug: slug}, summary)
	if err != nil {
		return fmt.Errorf("patch summary: %w", err)
	}
	if err := c.CheckStatus(patchResp.StatusCode(), patchResp.Body); err != nil {
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
