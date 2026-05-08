package agent

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// agentAuditCmd renders the events of one agent session for debugging.
//
// `dx agent audit <id>` resolves <id> as either the session_id text or the
// agent alias (most-recent for the alias), then pages through
// zdx_claude_events in seq order and pretty-prints them per turn so an
// operator can read what the agent saw and produced without piping psql
// output through jq.
//
// The bad-results debugging flow this exists for: spot whether the agent
// was given the right system prompt, whether tool_results came back as
// errors, whether the model emitted a coherent assistant turn, where it
// stalled, and whether the session ended in a value-producing or banned
// terminal state.
func agentAuditCmd() *cobra.Command {
	var asJSON bool
	var since int32
	var follow bool
	cmd := &cobra.Command{
		Use:   "audit <session-id-or-alias>",
		Short: "Pretty-print events for one agent session (resolves session-id text or agent alias)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c := cli.MustClient()
			slug := c.SlugOrDie()

			var resolved struct {
				ID         int64  `json:"id"`
				IssueID    string `json:"issue_id"`
				SessionID  string `json:"session_id"`
				Title      string `json:"title"`
				Alias      string `json:"alias"`
				Status     string `json:"status"`
				Lifecycle  string `json:"lifecycle"`
				EventCount int64  `json:"event_count"`
				CreatedAt  string `json:"created_at"`
				UpdatedAt  string `json:"updated_at"`
				ClosedAt   string `json:"closed_at"`
			}
			if err := c.Get("/api/dx/agent/audit/resolve",
				url.Values{"slug": {slug}, "id": {args[0]}}, &resolved); err != nil {
				return fmt.Errorf("resolve %q: %w", args[0], err)
			}

			if !asJSON {
				printSessionHeader(resolved.ID, resolved.SessionID, resolved.Alias, resolved.IssueID, resolved.Title, resolved.Status, resolved.Lifecycle, resolved.EventCount, resolved.CreatedAt, resolved.ClosedAt)
			}

			// Page through events. The server endpoint paginates; pull a wide
			// page since these are read-only and the operator wants the
			// whole session.
			const pageSize = 1000
			offset := int32(0)
			turnIdx := 0
			lastEventTime := time.Time{}
			lastSeqSeen := int32(-1)
			for {
				var page struct {
					Events []struct {
						Seq              int32           `json:"seq"`
						EventType        string          `json:"event_type"`
						EventJSON        json.RawMessage `json:"event_json"`
						CreatedAt        string          `json:"created_at"`
						AgentID          string          `json:"agent_id"`
						AgentType        string          `json:"agent_type"`
						AgentDescription string          `json:"agent_description"`
						IsSidechain      bool            `json:"is_sidechain"`
					} `json:"events"`
					Total int64 `json:"total"`
				}
				params := url.Values{
					"slug":   {slug},
					"limit":  {fmt.Sprint(pageSize)},
					"offset": {fmt.Sprint(offset)},
				}
				path := fmt.Sprintf("/api/dx/claude/sessions/%d/events", resolved.ID)
				if err := c.Get(path, params, &page); err != nil {
					return fmt.Errorf("fetch events offset=%d: %w", offset, err)
				}
				if len(page.Events) == 0 {
					break
				}
				for _, ev := range page.Events {
					if ev.Seq < since {
						continue
					}
					if asJSON {
						line, _ := json.Marshal(map[string]any{
							"seq":          ev.Seq,
							"event_type":   ev.EventType,
							"event_json":   ev.EventJSON,
							"created_at":   ev.CreatedAt,
							"agent_id":     ev.AgentID,
							"agent_type":   ev.AgentType,
							"is_sidechain": ev.IsSidechain,
						})
						fmt.Fprintln(os.Stdout, string(line))
						continue
					}
					t, _ := time.Parse(time.RFC3339Nano, ev.CreatedAt)
					if isTurnBoundary(ev.EventType, ev.EventJSON) {
						turnIdx++
						gap := ""
						if !lastEventTime.IsZero() {
							gap = fmt.Sprintf(" (+%s)", t.Sub(lastEventTime).Round(time.Millisecond))
						}
						fmt.Printf("\n─── turn %d  %s%s ───\n", turnIdx, t.Format("15:04:05"), gap)
					}
					lastEventTime = t
					lastSeqSeen = ev.Seq
					fmt.Println(renderAuditEvent(ev.Seq, ev.EventType, ev.EventJSON, ev.IsSidechain, ev.AgentID))
				}
				offset += int32(len(page.Events))
				if int64(offset) >= page.Total {
					break
				}
			}

			if !follow {
				return nil
			}

			// --follow: connect to the SSE endpoint, resume from the
			// highest seq we actually rendered during backfill so no
			// event prints twice and no gap is missed if the session
			// emitted while we were paging. (offset != seq — the
			// existing events endpoint paginates by row offset, while
			// the SSE cursor is the seq column.)
			if !asJSON {
				fmt.Fprintln(os.Stderr, "── following live (Ctrl-C to stop) ──")
			}
			return c.GetStreamSSE(cmd.Context(), "/api/dx/agent/audit/stream",
				url.Values{
					"slug":  {slug},
					"id":    {args[0]},
					"since": {fmt.Sprint(lastSeqSeen)},
				},
				fmt.Sprint(lastSeqSeen),
				func(raw []byte) error {
					var ev struct {
						Seq         int32           `json:"seq"`
						EventType   string          `json:"event_type"`
						EventJSON   json.RawMessage `json:"event_json"`
						CreatedAt   string          `json:"created_at"`
						AgentID     string          `json:"agent_id"`
						AgentType   string          `json:"agent_type"`
						IsSidechain bool            `json:"is_sidechain"`
					}
					if err := json.Unmarshal(raw, &ev); err != nil {
						return nil // skip malformed frames silently — server may emit pings
					}
					if asJSON {
						fmt.Println(string(raw))
						return nil
					}
					t, _ := time.Parse(time.RFC3339Nano, ev.CreatedAt)
					if isTurnBoundary(ev.EventType, ev.EventJSON) {
						turnIdx++
						gap := ""
						if !lastEventTime.IsZero() {
							gap = fmt.Sprintf(" (+%s)", t.Sub(lastEventTime).Round(time.Millisecond))
						}
						fmt.Printf("\n─── turn %d  %s%s ───\n", turnIdx, t.Format("15:04:05"), gap)
					}
					lastEventTime = t
					fmt.Println(renderAuditEvent(ev.Seq, ev.EventType, ev.EventJSON, ev.IsSidechain, ev.AgentID))
					return nil
				})
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "emit raw event JSON one per line instead of pretty-printing")
	cmd.Flags().Int32Var(&since, "since", 0, "only print events with seq >= this value")
	cmd.Flags().BoolVar(&follow, "follow", false, "after backfill, tail new events live via SSE (Ctrl-C to exit)")
	return cmd
}

// printSessionHeader emits a one-block summary at the top of an audit run
// so the operator immediately sees which session is being read, who owns
// it, and how big it is.
func printSessionHeader(id int64, sessionID, alias, issueID, title, status, lifecycle string, eventCount int64, createdAt, closedAt string) {
	closed := closedAt
	if closed == "" {
		closed = "<open>"
	}
	if alias == "" {
		alias = "-"
	}
	if issueID == "" {
		issueID = "-"
	}
	fmt.Printf("session_pk=%d  session_id=%s\n", id, sessionID)
	fmt.Printf("alias=%s  issue=%s  status=%s  lifecycle=%s\n", alias, issueID, status, lifecycle)
	fmt.Printf("events=%d  created=%s  closed=%s\n", eventCount, createdAt, closed)
	if title != "" {
		fmt.Printf("title=%s\n", title)
	}
}

// isTurnBoundary returns true when the given event marks the start of a
// new conversational turn — used to insert turn dividers in the output.
// User text and assistant text both start a turn (the model alternates
// between them); tool_call / tool_result are continuations of the prior
// turn.
func isTurnBoundary(eventType string, eventJSON json.RawMessage) bool {
	switch eventType {
	case "user", "user_text":
		return true
	case "assistant", "assistant_text":
		return true
	}
	// Some emitters use "type" inside event_json — peek at it.
	var probe struct {
		Type    string `json:"type"`
		Message struct {
			Role string `json:"role"`
		} `json:"message"`
	}
	if json.Unmarshal(eventJSON, &probe) == nil {
		if probe.Type == "user" || probe.Type == "assistant" {
			return true
		}
		if probe.Message.Role == "user" || probe.Message.Role == "assistant" {
			return true
		}
	}
	return false
}

// renderAuditEvent produces the one-line pretty form for a single event.
// Lines are prefixed with a 4-letter type tag and the seq number so the
// operator can grep by tag and locate by seq.
func renderAuditEvent(seq int32, eventType string, eventJSON json.RawMessage, isSidechain bool, agentID string) string {
	tag, body := classifyAuditEvent(eventType, eventJSON)
	side := ""
	if isSidechain {
		side = " (sub)"
	}
	if agentID != "" && tag != "USR" && tag != "AST" {
		// Agent attribution matters for tool calls and side events but
		// would be noise on user/assistant text.
		side = side + " [" + agentID + "]"
	}
	return fmt.Sprintf("[%s%s seq=%-3d] %s", tag, side, seq, body)
}

// classifyAuditEvent maps an event_type + payload onto a 3-char tag and a
// one-line summary body. The intent is operator-readable: tool_calls show
// the tool name and a short input preview; tool_results show ok/err + a
// size; assistant text is truncated.
func classifyAuditEvent(eventType string, eventJSON json.RawMessage) (string, string) {
	switch eventType {
	case "compaction.applied", "tombstone.applied":
		var rec struct {
			Strategy         string `json:"strategy"`
			EventID          string `json:"event_id"`
			OriginalSize     int    `json:"original_size"`
			CompactedContent string `json:"compacted_content"`
		}
		_ = json.Unmarshal(eventJSON, &rec)
		return "CMP", fmt.Sprintf("%s strategy=%s event_id=%s orig=%dB", eventType, rec.Strategy, rec.EventID, rec.OriginalSize)
	case "ai-title", "ai_title":
		var rec struct {
			Title string `json:"title"`
		}
		_ = json.Unmarshal(eventJSON, &rec)
		return "TTL", rec.Title
	case "file_edit":
		var rec struct {
			Tool  string          `json:"tool"`
			Input json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(eventJSON, &rec)
		return "EDT", fmt.Sprintf("%s %s", rec.Tool, truncate(string(rec.Input), 200))
	case "shell_cmd":
		var rec struct {
			Tool  string          `json:"tool"`
			Input json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(eventJSON, &rec)
		return "BSH", fmt.Sprintf("%s %s", rec.Tool, truncate(string(rec.Input), 200))
	case "tool_call":
		var rec struct {
			Tool  string          `json:"tool"`
			Input json.RawMessage `json:"input"`
		}
		_ = json.Unmarshal(eventJSON, &rec)
		return "TC ", fmt.Sprintf("%s %s", rec.Tool, truncate(string(rec.Input), 200))
	}

	// Fall through to message-shaped events. The opencode adapter writes
	// "type":"user"/"assistant" with a message.content array; the claude
	// adapter writes a similar shape.
	var msg struct {
		Type    string `json:"type"`
		Message struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
			Usage   json.RawMessage `json:"usage"`
		} `json:"message"`
	}
	_ = json.Unmarshal(eventJSON, &msg)

	role := msg.Message.Role
	if role == "" {
		role = msg.Type
	}
	if role == "" {
		role = eventType
	}

	// Content can be a plain string or an array of content blocks.
	contentText, blocks := extractContentBlocks(msg.Message.Content)

	switch role {
	case "user":
		// User-role events come from two sources: the operator's initial
		// prompt (plain text) and tool_result blocks (which carry the
		// tool output back to the model). Disambiguate by looking at
		// the first block's type.
		if len(blocks) > 0 && blocks[0].Type == "tool_result" {
			isErr := blocks[0].IsError
			tag := "TR "
			if isErr {
				tag = "TRX"
			}
			return tag, fmt.Sprintf("tool_use_id=%s len=%dB %s",
				blocks[0].ToolUseID, len(blocks[0].Content),
				truncate(blocks[0].Content, 200))
		}
		return "USR", truncate(contentText, 240)
	case "assistant":
		// Assistant turns may carry text + tool_use blocks. Render both
		// in a compact shape so one line tells you "the model said X
		// and called Y(args=...)".
		var parts []string
		if contentText != "" {
			parts = append(parts, truncate(contentText, 200))
		}
		for _, b := range blocks {
			if b.Type == "tool_use" {
				parts = append(parts, fmt.Sprintf("→ %s(%s)", b.Name, truncate(string(b.Input), 120)))
			}
		}
		body := strings.Join(parts, "  ")
		if body == "" {
			body = "(empty)"
		}
		return "AST", body
	}

	// Unknown shape — dump a truncated raw form so the operator can
	// still see something. This is the diagnostic backstop: any new
	// event_type that lands in the DB without a renderer-side case
	// shows up as "??" with its raw JSON, not silently dropped.
	return "?? ", fmt.Sprintf("%s %s", eventType, truncate(string(eventJSON), 200))
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   string          `json:"content"`
	IsError   bool            `json:"is_error"`
}

// extractContentBlocks accepts a message.content payload that's either a
// plain JSON string ("hi there") OR an array of typed blocks (as Claude /
// opencode emit them) and returns the flattened plain text plus the
// structured block list. Either return value can be empty.
func extractContentBlocks(raw json.RawMessage) (string, []contentBlock) {
	if len(raw) == 0 {
		return "", nil
	}
	var asString string
	if json.Unmarshal(raw, &asString) == nil {
		return asString, nil
	}
	var asBlocks []contentBlock
	if json.Unmarshal(raw, &asBlocks) == nil {
		var text []string
		for _, b := range asBlocks {
			if b.Type == "text" && b.Text != "" {
				text = append(text, b.Text)
			}
		}
		return strings.Join(text, "\n"), asBlocks
	}
	return "", nil
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
