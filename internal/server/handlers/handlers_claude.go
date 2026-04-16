package handlers

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
)

func (h *Handler) registerClaudeRoutes(api huma.API) {
	type ClaudeSessionItem struct {
		ID         int64  `json:"id"`
		IssueID    string `json:"issue_id"`
		SessionID  string `json:"session_id"`
		Title      string `json:"title"`
		Alias      string `json:"alias"`
		Header     string `json:"header"`
		Summary    string `json:"summary"`
		Status     string `json:"status"`
		EventCount int64  `json:"event_count"`
		CreatedAt  string `json:"created_at"`
	}

	type ClaudeEventItem struct {
		ID               int64           `json:"id"`
		Seq              int32           `json:"seq"`
		EventType        string          `json:"event_type"`
		EventJSON        json.RawMessage `json:"event_json"`
		CreatedAt        string          `json:"created_at"`
		AgentID          string          `json:"agent_id"`
		AgentType        string          `json:"agent_type"`
		AgentDescription string          `json:"agent_description"`
		IsSidechain      bool            `json:"is_sidechain"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-claude-sessions", Method: http.MethodGet, Path: "/api/dx/claude/sessions"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id"`
			Limit   int32  `query:"limit"`
			Offset  int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Sessions []ClaudeSessionItem `json:"sessions"`
				Total    int64               `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}

			var total int64
			var out []ClaudeSessionItem

			if in.IssueID != "" {
				total, _ = h.Q.CountClaudeSessionsByIssue(ctx, db.CountClaudeSessionsByIssueParams{ProjectID: p.ID, IssueID: in.IssueID})
				rows, err := h.Q.ListClaudeSessionsByIssue(ctx, db.ListClaudeSessionsByIssueParams{ProjectID: p.ID, IssueID: in.IssueID})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				out = make([]ClaudeSessionItem, len(rows))
				for i, r := range rows {
					cnt, _ := h.Q.CountClaudeEvents(ctx, r.ID)
					out[i] = ClaudeSessionItem{ID: r.ID, IssueID: r.IssueID, SessionID: r.SessionID, Title: r.Title, Alias: r.Alias, Header: r.Header, Summary: r.Summary, Status: r.Status, EventCount: cnt, CreatedAt: fmtTS(r.CreatedAt)}
				}
			} else {
				total, _ = h.Q.CountClaudeSessions(ctx, p.ID)
				limit, offset := parsePage(in.Limit, in.Offset)
				rows, err := h.Q.ListClaudeSessionsPaginated(ctx, db.ListClaudeSessionsPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
				out = make([]ClaudeSessionItem, len(rows))
				for i, r := range rows {
					cnt, _ := h.Q.CountClaudeEvents(ctx, r.ID)
					out[i] = ClaudeSessionItem{ID: r.ID, IssueID: r.IssueID, SessionID: r.SessionID, Title: r.Title, Alias: r.Alias, Header: r.Header, Summary: r.Summary, Status: r.Status, EventCount: cnt, CreatedAt: fmtTS(r.CreatedAt)}
				}
			}
			return &struct {
				Body struct {
					Sessions []ClaudeSessionItem `json:"sessions"`
					Total    int64               `json:"total"`
				}
			}{Body: struct {
				Sessions []ClaudeSessionItem `json:"sessions"`
				Total    int64               `json:"total"`
			}{Sessions: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-claude-session", Method: http.MethodGet, Path: "/api/dx/claude/sessions/{sessionId}"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
		}) (*struct {
			Body ClaudeSessionItem
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			cnt, _ := h.Q.CountClaudeEvents(ctx, sess.ID)
			return &struct {
				Body ClaudeSessionItem
			}{Body: ClaudeSessionItem{
				ID:         sess.ID,
				IssueID:    sess.IssueID,
				SessionID:  sess.SessionID,
				Title:      sess.Title,
				Alias:      sess.Alias,
				Header:     sess.Header,
				Summary:    sess.Summary,
				Status:     sess.Status,
				EventCount: cnt,
				CreatedAt:  fmtTS(sess.CreatedAt),
			}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-claude-session-events", Method: http.MethodGet, Path: "/api/dx/claude/sessions/{sessionId}/events"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
			Limit     int32  `query:"limit"`
			Offset    int32  `query:"offset"`
		}) (*struct {
			Body struct {
				Events []ClaudeEventItem `json:"events"`
				Total  int64             `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			total, _ := h.Q.CountClaudeEvents(ctx, sess.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := h.Q.ListClaudeEventsPaginated(ctx, db.ListClaudeEventsPaginatedParams{
				SessionPk: sess.ID,
				Limit:     limit,
				Offset:    offset,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ClaudeEventItem, len(rows))
			for i, r := range rows {
				out[i] = ClaudeEventItem{
					ID:               r.ID,
					Seq:              r.Seq,
					EventType:        r.EventType,
					EventJSON:        json.RawMessage(r.EventJson),
					CreatedAt:        fmtTS(r.CreatedAt),
					AgentID:          r.AgentID,
					AgentType:        r.AgentType,
					AgentDescription: r.AgentDescription,
					IsSidechain:      r.IsSidechain,
				}
			}
			return &struct {
				Body struct {
					Events []ClaudeEventItem `json:"events"`
					Total  int64             `json:"total"`
				}
			}{Body: struct {
				Events []ClaudeEventItem `json:"events"`
				Total  int64             `json:"total"`
			}{Events: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-claude-session-token-usage", Method: http.MethodGet, Path: "/api/dx/claude/sessions/{sessionId}/token-usage"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
		}) (*struct {
			Body struct {
				InputTokens              int64 `json:"input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			usage, err := h.Q.GetClaudeSessionTokenUsage(ctx, sess.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				}
			}{Body: struct {
				InputTokens              int64 `json:"input_tokens"`
				OutputTokens             int64 `json:"output_tokens"`
				CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
				CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
			}{
				InputTokens:              usage.InputTokens,
				OutputTokens:             usage.OutputTokens,
				CacheReadInputTokens:     usage.CacheReadInputTokens,
				CacheCreationInputTokens: usage.CacheCreationInputTokens,
			}}, nil
		})

	type AgentTokenUsageRow struct {
		AgentID                  string `json:"agent_id"`
		AgentType                string `json:"agent_type"`
		AgentDescription         string `json:"agent_description"`
		InputTokens              int64  `json:"input_tokens"`
		OutputTokens             int64  `json:"output_tokens"`
		CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
		CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
		EventCount               int64  `json:"event_count"`
	}

	huma.Register(api, huma.Operation{OperationID: "get-claude-session-token-usage-by-agent", Method: http.MethodGet, Path: "/api/dx/claude/sessions/{sessionId}/token-usage/by-agent"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
		}) (*struct {
			Body struct {
				Agents []AgentTokenUsageRow `json:"agents"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			rows, err := h.Q.GetClaudeSessionTokenUsageByAgent(ctx, sess.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			agents := make([]AgentTokenUsageRow, len(rows))
			for i, r := range rows {
				agents[i] = AgentTokenUsageRow{
					AgentID:                  r.AgentID,
					AgentType:                r.AgentType,
					AgentDescription:         r.AgentDescription,
					InputTokens:              r.InputTokens,
					OutputTokens:             r.OutputTokens,
					CacheReadInputTokens:     r.CacheReadInputTokens,
					CacheCreationInputTokens: r.CacheCreationInputTokens,
					EventCount:               r.EventCount,
				}
			}
			return &struct {
				Body struct {
					Agents []AgentTokenUsageRow `json:"agents"`
				}
			}{Body: struct {
				Agents []AgentTokenUsageRow `json:"agents"`
			}{Agents: agents}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-claude-session-summary", Method: http.MethodPatch, Path: "/api/dx/claude/sessions/{sessionId}/summary"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
			Body      struct {
				Header  string `json:"header"`
				Summary string `json:"summary"`
				Status  string `json:"status"`
			}
		}) (*struct {
			Body struct {
				OK bool `json:"ok"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.UpdateClaudeSessionSummary(ctx, db.UpdateClaudeSessionSummaryParams{
				ProjectID: p.ID,
				ID:        in.SessionID,
				Header:    in.Body.Header,
				Summary:   in.Body.Summary,
				Status:    in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			if h.Broker.IsWSEnabled() {
				sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
				if err == nil {
					payload := map[string]string{
						"header":  in.Body.Header,
						"summary": in.Body.Summary,
						"status":  in.Body.Status,
					}
					h.Broker.PublishAgentSessionLifecycle(in.Slug, sess.SessionID, "agent.session-updated", payload)
					h.Broker.PublishClaudeEvent(in.Slug, sess.SessionID, "claude.session-updated", payload)
				}
			}
			return &struct {
				Body struct {
					OK bool `json:"ok"`
				}
			}{Body: struct {
				OK bool `json:"ok"`
			}{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-churn-sessions", Method: http.MethodGet, Path: "/api/dx/claude/sessions/churns"},
		func(ctx context.Context, in *struct {
			Slug  string `query:"slug" required:"true"`
			Since string `query:"since"`
		}) (*struct {
			Body struct {
				Sessions []ClaudeSessionItem `json:"sessions"`
				Total    int64               `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			since := pgtype.Timestamptz{}
			if in.Since != "" {
				if t, err := time.Parse(time.RFC3339, in.Since); err == nil {
					since = pgtype.Timestamptz{Time: t, Valid: true}
				}
			}
			if !since.Valid {
				since = pgtype.Timestamptz{Time: time.Now().AddDate(0, 0, -7), Valid: true}
			}
			total, _ := h.Q.CountChurnSessions(ctx, db.CountChurnSessionsParams{ProjectID: p.ID, CreatedAt: since})
			rows, err := h.Q.ListChurnSessions(ctx, db.ListChurnSessionsParams{ProjectID: p.ID, CreatedAt: since})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ClaudeSessionItem, len(rows))
			for i, r := range rows {
				cnt, _ := h.Q.CountClaudeEvents(ctx, r.ID)
				out[i] = ClaudeSessionItem{ID: r.ID, IssueID: r.IssueID, SessionID: r.SessionID, Title: r.Title, Alias: r.Alias, Header: r.Header, Summary: r.Summary, Status: r.Status, EventCount: cnt, CreatedAt: fmtTS(r.CreatedAt)}
			}
			return &struct {
				Body struct {
					Sessions []ClaudeSessionItem `json:"sessions"`
					Total    int64               `json:"total"`
				}
			}{Body: struct {
				Sessions []ClaudeSessionItem `json:"sessions"`
				Total    int64               `json:"total"`
			}{Sessions: out, Total: total}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "extract-pattern-from-session", Method: http.MethodPost, Path: "/api/dx/claude/sessions/{sessionId}/extract-pattern"},
		func(ctx context.Context, in *struct {
			Slug      string `query:"slug" required:"true"`
			SessionID int64  `path:"sessionId"`
		}) (*struct {
			Body PatternItem
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			if sess.Status != "churn" {
				return nil, apiErr(400, "session is not a churn session")
			}

			events, err := h.Q.ListClaudeEvents(ctx, sess.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}

			var transcript strings.Builder
			for _, ev := range events {
				var parsed map[string]any
				if json.Unmarshal(ev.EventJson, &parsed) != nil {
					continue
				}
				msg, _ := parsed["message"].(map[string]any)
				if msg == nil {
					continue
				}
				role, _ := msg["role"].(string)
				if role != "user" && role != "assistant" {
					continue
				}
				content := extractSessionTextContent(msg["content"])
				if content == "" {
					continue
				}
				if len(content) > 2000 {
					content = content[:2000] + "..."
				}
				transcript.WriteString(fmt.Sprintf("[%s] %s\n\n", role, content))
			}

			if transcript.Len() == 0 {
				return nil, apiErr(400, "session has no analyzable content")
			}

			prompt := `Analyze this Claude Code session that was classified as "churn" (repeated edits, going in circles, retrying failing approaches).

Extract a reusable pattern that captures:
1. What went wrong (the root cause of the churn)
2. How to avoid it in future sessions

Return JSON with exactly two fields:
- "name": a short, descriptive name for the pattern (e.g. "missing-migration-regen", "circular-type-fix")
- "description": 2-4 sentences: what the failure pattern is, why it causes churn, and the recommended approach to avoid it

Return ONLY valid JSON, no markdown fences, no explanation.

Session header: ` + sess.Header + `
Session summary: ` + sess.Summary + `

Transcript:
` + transcript.String()

			result, err := h.Emb.Complete(ctx, []llm.ChatMessage{
				{Role: "user", Content: prompt},
			})
			if err != nil {
				return nil, apiErr(500, "llm extraction failed: "+err.Error())
			}

			result = strings.TrimSpace(result)
			result = strings.TrimPrefix(result, "```json")
			result = strings.TrimPrefix(result, "```")
			result = strings.TrimSuffix(result, "```")
			result = strings.TrimSpace(result)

			var extracted struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			if err := json.Unmarshal([]byte(result), &extracted); err != nil {
				return nil, apiErr(500, "failed to parse LLM response")
			}

			row, err := h.Q.InsertPattern(ctx, db.InsertPatternParams{
				ProjectID:   p.ID,
				Name:        extracted.Name,
				Description: extracted.Description,
				CodeRefs:    json.RawMessage("[]"),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertPattern(context.Background(), p.ID, row.ID, row.Name+" "+row.Description)
			return &struct{ Body PatternItem }{Body: toPatternItem(row)}, nil
		})

	// Streaming ingest: legacy alias. Preserved as a backward-compat entry
	// point for older dx-cli builds that POST raw JSONL; delegates to the
	// unified ingestion path via shared helpers (getOrCreateAgentSession,
	// ingestAgentEvent). Will be removed once deployed agents upgrade.
	h.Mux.Post("/api/dx/claude/sessions/ingest/stream", h.handleClaudeSessionIngestStream)
}

// ── Claude session streaming ingest (legacy) ─────────────────────────────
//
// Accepts raw adapter JSONL (one event per line, adapter-native schema) via
// query params. Aliased to the unified /api/dx/agent/sessions/{sid}/events
// path: uses the same getOrCreateAgentSession + ingestAgentEvent helpers so
// event persistence and WS publishes follow a single code path. A later task
// will remove this endpoint and migrate clients to /api/dx/agent/sessions.

func (h *Handler) handleClaudeSessionIngestStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	slug := r.URL.Query().Get("slug")
	sessionUUID := r.URL.Query().Get("session_id")
	issueID := r.URL.Query().Get("issue_id")
	alias := r.URL.Query().Get("alias")
	agentID := r.URL.Query().Get("agent_id")
	agentType := r.URL.Query().Get("agent_type")
	agentDesc := r.URL.Query().Get("agent_description")
	isSidechain := agentID != ""

	if slug == "" || sessionUUID == "" {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"slug and session_id required"}`, http.StatusBadRequest)
		return
	}

	p, err := h.Q.GetProjectBySlug(ctx, slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"project not found"}`, http.StatusNotFound)
		return
	}

	scanner := bufio.NewScanner(r.Body)
	scanner.Buffer(make([]byte, 1<<20), 4<<20)

	var sess db.CreateClaudeSessionRow
	sessionReady := false
	seq := int32(0)
	var allLines []string

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		if !sessionReady {
			title := extractTitleFromLegacyLine(line)
			s2, created, cErr := h.getOrCreateAgentSession(ctx, p.ID, sessionUUID, issueID, alias, title)
			if cErr != nil {
				http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
				return
			}
			sess = s2
			if !created {
				cnt, cntErr := h.Q.CountClaudeEvents(ctx, sess.ID)
				if cntErr == nil {
					seq = int32(cnt)
				}
			} else {
				h.Broker.PublishAgentSessionLifecycle(slug, sess.SessionID, "agent.session-created", map[string]any{
					"session_id": sess.SessionID,
					"session_pk": sess.ID,
					"title":      title,
					"alias":      alias,
				})
				// Legacy event name for subscribers that have not migrated.
				h.Broker.PublishClaudeSessionLifecycle(slug, sess.SessionID, "claude.session-created", map[string]any{
					"session_id": sess.SessionID,
					"session_pk": sess.ID,
					"title":      title,
					"alias":      alias,
				})
			}
			sessionReady = true
		}

		eventType := ""
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) == nil {
			if t, ok := ev["type"].(string); ok {
				eventType = t
			}
		}

		h.ingestAgentEvent(ctx, slug, sess.SessionID, sess.ID, seq, eventType, []byte(line), agentID, agentType, agentDesc)
		allLines = append(allLines, line)
		seq++
	}

	if !sessionReady {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"empty body"}`, http.StatusBadRequest)
		return
	}

	h.Broker.PublishAgentSessionLifecycle(slug, sess.SessionID, "agent.session-closed", map[string]any{
		"session_id":  sess.SessionID,
		"session_pk":  sess.ID,
		"event_count": seq,
	})
	h.Broker.PublishClaudeSessionLifecycle(slug, sess.SessionID, "claude.session-closed", map[string]any{
		"session_id":  sess.SessionID,
		"session_pk":  sess.ID,
		"event_count": seq,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         sess.ID,
		"session_id": sess.SessionID,
		"events":     seq,
	})

	if !isSidechain {
		go h.summarizeSessionAsync(p.ID, sess.ID, allLines)
	}
}

// extractTitleFromLegacyLine parses a Claude-adapter ai-title event to pull
// out the one-line session title. Returns empty string when the line is not
// an ai-title event.
func extractTitleFromLegacyLine(line string) string {
	var ev map[string]any
	if json.Unmarshal([]byte(line), &ev) != nil {
		return ""
	}
	if ev["type"] != "ai-title" {
		return ""
	}
	if msg, ok := ev["message"].(map[string]any); ok {
		if t, ok := msg["content"].(string); ok && t != "" {
			return t
		}
	}
	if t, ok := ev["title"].(string); ok {
		return t
	}
	return ""
}

func (h *Handler) summarizeSessionAsync(projectID int32, sessionPK int64, lines []string) {
	ctx := WithSource(context.Background(), "auto-summarize")

	var transcript strings.Builder
	for _, line := range lines {
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		msg, _ := ev["message"].(map[string]any)
		if msg == nil {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "user" && role != "assistant" {
			continue
		}
		content := extractSessionTextContent(msg["content"])
		if content == "" {
			continue
		}
		if len(content) > 2000 {
			content = content[:2000] + "..."
		}
		transcript.WriteString(fmt.Sprintf("[%s] %s\n\n", role, content))
	}

	if transcript.Len() == 0 {
		return
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

	result, err := h.Emb.Complete(ctx, []llm.ChatMessage{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		log.Printf("auto-summarize session %d: llm complete: %v", sessionPK, err)
		return
	}

	result = strings.TrimSpace(result)
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
		log.Printf("auto-summarize session %d: parse response: %v\nraw: %s", sessionPK, err, result)
		return
	}

	if summary.Status != "ok" && summary.Status != "churn" && summary.Status != "errored" {
		summary.Status = "ok"
	}

	if err := h.Q.UpdateClaudeSessionSummary(ctx, db.UpdateClaudeSessionSummaryParams{
		ProjectID: projectID,
		ID:        sessionPK,
		Header:    summary.Header,
		Summary:   summary.Summary,
		Status:    summary.Status,
	}); err != nil {
		log.Printf("auto-summarize session %d: update db: %v", sessionPK, err)
		return
	}

	if h.Broker.IsWSEnabled() {
		sess, err := h.Q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: projectID, ID: sessionPK})
		if err == nil {
			p, pErr := h.Q.GetProjectByID(ctx, projectID)
			if pErr == nil {
				payload := map[string]string{
					"header":  summary.Header,
					"summary": summary.Summary,
					"status":  summary.Status,
				}
				h.Broker.PublishAgentSessionLifecycle(p.Slug, sess.SessionID, "agent.session-updated", payload)
				h.Broker.PublishClaudeEvent(p.Slug, sess.SessionID, "claude.session-updated", payload)
			}
		}
	}
}

func extractSessionTextContent(content any) string {
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
