package devtools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// LogCmd groups log-event observability subcommands.
func LogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Tail and inspect zdx_log_events (tracelog stream)",
	}
	cmd.AddCommand(logTailCmd())
	return cmd
}

// logTailCmd connects to /api/dx/log-events/stream over SSE and prints
// events as they arrive. The server emits one SSE frame per event:
//
//	id: 2026-...Z   <- RFC3339Nano timestamp; client sends as Last-Event-ID on reconnect
//	data: {"id":42,"level":"info","message":"...","context_json":{...},...}
//
// Plus periodic `:heartbeat` comments to keep idle-aware proxies (HAProxy)
// from closing the connection.
//
// Reconnect: on read error, retries with exponential backoff. Last-Event-ID
// preserves cursor across reconnects so events aren't duplicated or missed.
func logTailCmd() *cobra.Command {
	var tagPairs []string
	var since string
	var jsonOut bool
	var levelFilter string
	cmd := &cobra.Command{
		Use:   "tail",
		Short: "Tail log events as a JSON stream, filtered by tag (SSE)",
		Long: `Connects to /api/dx/log-events/stream over Server-Sent Events and prints
new events as they arrive. Filter via repeatable --tag KEY=VALUE.

Examples:
  dx log tail --tag alias=agent-XYZ
  dx log tail --tag alias=agent-XYZ --tag iteration_id=abc123
  dx log tail --tag agent=claude --level=warn
  dx log tail --since=2026-05-05T14:00:00Z --json

Sub-second latency under normal load. Heartbeat (:heartbeat SSE comment)
fires every 15s when no real events flow so the connection survives
HAProxy / nginx idle timeouts.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			tagFilter, err := buildTagFilter(tagPairs)
			if err != nil {
				return err
			}
			c := cli.MustClient()
			slug := c.SlugOrDie()

			cursor := ""
			if since != "" {
				if t, err := time.Parse(time.RFC3339, since); err == nil {
					cursor = t.UTC().Format(time.RFC3339Nano)
				} else {
					return fmt.Errorf("--since: %w", err)
				}
			}

			fmt.Fprintf(os.Stderr, "[log tail] tag_filter=%s since=%s slug=%s\n",
				summarizeTagFilter(tagFilter), summarizeCursor(cursor), slug)

			backoff := 1 * time.Second
			for {
				if err := streamOnce(cmd.Context(), c, slug, tagFilter, &cursor, levelFilter, jsonOut); err != nil {
					if cmd.Context().Err() != nil {
						return nil
					}
					fmt.Fprintf(os.Stderr, "[log tail] stream error: %v — reconnecting in %s\n", err, backoff)
					select {
					case <-cmd.Context().Done():
						return nil
					case <-time.After(backoff):
					}
					if backoff < 30*time.Second {
						backoff *= 2
					}
					continue
				}
				return nil
			}
		},
	}
	cmd.Flags().StringArrayVar(&tagPairs, "tag", nil, "tag filter as KEY=VALUE (repeatable; ANDed)")
	cmd.Flags().StringVar(&since, "since", "", "RFC3339 timestamp to start from (default: server's now-1s)")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "emit raw JSON per event instead of formatted line")
	cmd.Flags().StringVar(&levelFilter, "level", "", "client-side level filter (debug|info|warn|error)")
	return cmd
}

// buildTagFilter turns ["k=v", "k2=v2"] into '{"k":"v","k2":"v2"}'.
// The server applies this as a JSONB @> on context_json, so multiple tags
// behave as AND (each key must be present with the given value).
func buildTagFilter(pairs []string) (string, error) {
	if len(pairs) == 0 {
		return "", nil
	}
	m := map[string]string{}
	for _, p := range pairs {
		idx := strings.Index(p, "=")
		if idx <= 0 {
			return "", fmt.Errorf("--tag must be KEY=VALUE, got %q", p)
		}
		m[p[:idx]] = p[idx+1:]
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// streamOnce opens a single SSE connection and reads frames until the
// context cancels or the connection drops. Updates *cursor with the last
// event's id so a reconnect resumes from the right place.
func streamOnce(ctx context.Context, c *cli.Client, slug, tagFilter string, cursor *string, levelFilter string, jsonOut bool) error {
	q := url.Values{}
	q.Set("slug", slug)
	if tagFilter != "" {
		q.Set("tag_filter", tagFilter)
	}
	if *cursor != "" {
		q.Set("since", *cursor)
	}
	streamURL := strings.TrimRight(c.Base(), "/") + "/api/dx/log-events/stream?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, streamURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", c.Token())
	req.Header.Set("Accept", "text/event-stream")
	if *cursor != "" {
		req.Header.Set("Last-Event-ID", *cursor)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	br := bufio.NewReader(resp.Body)
	var dataBuf strings.Builder
	var lastID string
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			if ctx.Err() != nil || err == io.EOF {
				return nil
			}
			return err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			// Frame separator. Emit accumulated event.
			if dataBuf.Len() > 0 {
				emitEvent(dataBuf.String(), lastID, levelFilter, jsonOut)
				if lastID != "" {
					*cursor = lastID
				}
			}
			dataBuf.Reset()
			lastID = ""
			continue
		}
		switch {
		case strings.HasPrefix(line, ":"):
			// SSE comment / heartbeat — ignore.
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			if dataBuf.Len() > 0 {
				dataBuf.WriteByte('\n')
			}
			dataBuf.WriteString(payload)
		case strings.HasPrefix(line, "id:"):
			lastID = strings.TrimSpace(strings.TrimPrefix(line, "id:"))
		}
	}
}

// streamEvent is the wire-shape the server sends in `data:` lines.
type streamEvent struct {
	ID          int64           `json:"id"`
	Level       string          `json:"level"`
	Message     string          `json:"message"`
	Source      string          `json:"source"`
	Component   string          `json:"component"`
	Environment string          `json:"environment"`
	ContextJson json.RawMessage `json:"context_json"`
	CreatedAt   string          `json:"created_at"`
}

func emitEvent(payload, _, levelFilter string, jsonOut bool) {
	var ev streamEvent
	if err := json.Unmarshal([]byte(payload), &ev); err != nil {
		fmt.Fprintf(os.Stderr, "[log tail] decode: %v\n", err)
		return
	}
	if levelFilter != "" && ev.Level != levelFilter {
		return
	}
	if jsonOut {
		fmt.Println(payload)
		return
	}
	fmt.Println(formatEvent(ev))
}

// formatEvent renders one event as a single readable line:
//
//	HH:MM:SS.ms LEVEL message  alias=X iteration_id=Y todo_id=N ...
//
// Common tags print first; bookkeeping tags (trace_id, code.*, slug, pid,
// scope, branch, worktree, agent) are hidden by default to keep the line
// scannable. --json shows everything verbatim.
func formatEvent(ev streamEvent) string {
	ts := ev.CreatedAt
	if t, err := time.Parse(time.RFC3339Nano, ev.CreatedAt); err == nil {
		ts = t.Local().Format("15:04:05.000")
	}
	var tags map[string]any
	if len(ev.ContextJson) > 0 {
		_ = json.Unmarshal(ev.ContextJson, &tags)
	}

	priority := []string{"alias", "iteration_id", "todo_id", "todo_key", "issue_id", "session_id", "type"}
	hidden := map[string]bool{
		"trace_id": true, "code.file": true, "code.line": true, "code.func": true,
		"agent": true, "slug": true, "branch": true, "worktree": true, "pid": true, "scope": true,
	}

	var first []string
	seen := map[string]bool{}
	for _, k := range priority {
		if v, ok := tags[k]; ok {
			first = append(first, fmt.Sprintf("%s=%v", k, v))
			seen[k] = true
		}
	}
	var rest []string
	for k, v := range tags {
		if seen[k] || hidden[k] {
			continue
		}
		rest = append(rest, fmt.Sprintf("%s=%v", k, v))
	}
	sort.Strings(rest)
	tagStr := strings.Join(append(first, rest...), " ")

	level := strings.ToUpper(ev.Level)
	if level == "" {
		level = "INFO"
	}
	if tagStr == "" {
		return fmt.Sprintf("%s %-5s %s", ts, level, ev.Message)
	}
	return fmt.Sprintf("%s %-5s %-30s  %s", ts, level, ev.Message, tagStr)
}

func summarizeTagFilter(tagFilter string) string {
	if tagFilter == "" {
		return "(none)"
	}
	return tagFilter
}

func summarizeCursor(cursor string) string {
	if cursor == "" {
		return "(server's now-1s)"
	}
	return cursor
}
