package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
)

func (s *Server) registerClaudeRoutes(api huma.API) {
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
		ID        int64           `json:"id"`
		Seq       int32           `json:"seq"`
		EventType string          `json:"event_type"`
		EventJSON json.RawMessage `json:"event_json"`
		CreatedAt string          `json:"created_at"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-claude-sessions", Method: http.MethodGet, Path: "/api/dx/claude/sessions"},
		func(ctx context.Context, in *PaginatedSlugInput) (*struct {
			Body struct {
				Sessions []ClaudeSessionItem `json:"sessions"`
				Total    int64               `json:"total"`
			}
		}, error) {
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			total, _ := s.q.CountClaudeSessions(ctx, p.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListClaudeSessionsPaginated(ctx, db.ListClaudeSessionsPaginatedParams{ProjectID: p.ID, Limit: limit, Offset: offset})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ClaudeSessionItem, len(rows))
			for i, r := range rows {
				cnt, _ := s.q.CountClaudeEvents(ctx, r.ID)
				out[i] = ClaudeSessionItem{
					ID:         r.ID,
					IssueID:    r.IssueID,
					SessionID:  r.SessionID,
					Title:      r.Title,
					Alias:      r.Alias,
					Header:     r.Header,
					Summary:    r.Summary,
					Status:     r.Status,
					EventCount: cnt,
					CreatedAt:  fmtTS(r.CreatedAt),
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := s.q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			cnt, _ := s.q.CountClaudeEvents(ctx, sess.ID)
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := s.q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			total, _ := s.q.CountClaudeEvents(ctx, sess.ID)
			limit, offset := parsePage(in.Limit, in.Offset)
			rows, err := s.q.ListClaudeEventsPaginated(ctx, db.ListClaudeEventsPaginatedParams{
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
					ID:        r.ID,
					Seq:       r.Seq,
					EventType: r.EventType,
					EventJSON: json.RawMessage(r.EventJson),
					CreatedAt: fmtTS(r.CreatedAt),
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := s.q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			usage, err := s.q.GetClaudeSessionTokenUsage(ctx, sess.ID)
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			sess, err := s.q.GetClaudeSession(ctx, db.GetClaudeSessionParams{ProjectID: p.ID, ID: in.SessionID})
			if err != nil {
				return nil, apiErr(404, "session not found")
			}
			rows, err := s.q.GetClaudeSessionTokenUsageByAgent(ctx, sess.ID)
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.UpdateClaudeSessionSummary(ctx, db.UpdateClaudeSessionSummaryParams{
				ProjectID: p.ID,
				ID:        in.SessionID,
				Header:    in.Body.Header,
				Summary:   in.Body.Summary,
				Status:    in.Body.Status,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct {
				Body struct {
					OK bool `json:"ok"`
				}
			}{Body: struct {
				OK bool `json:"ok"`
			}{OK: true}}, nil
		})

	// Ingest endpoint: accepts JSONL body, creates session + events in one call.
	s.mux.Post("/api/dx/claude/sessions/ingest", s.handleClaudeSessionIngest)
}

// ── Claude session ingest ─────────────────────────────────────────────────

func (s *Server) handleClaudeSessionIngest(w http.ResponseWriter, r *http.Request) {
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

	p, err := s.q.GetProjectBySlug(ctx, slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"project not found"}`, http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, 50<<20)) // 50 MB max
	if err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"read body failed"}`, http.StatusBadRequest)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) == 0 {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"empty body"}`, http.StatusBadRequest)
		return
	}

	// Extract title from ai-title event if present.
	title := ""
	for _, line := range lines {
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) == nil {
			if ev["type"] == "ai-title" {
				if msg, ok := ev["message"].(map[string]any); ok {
					if t, ok := msg["content"].(string); ok {
						title = t
					}
				}
				if t, ok := ev["title"].(string); ok && title == "" {
					title = t
				}
			}
		}
	}

	var sessionPK int64
	var sessionID string

	sess, err := s.q.CreateClaudeSession(ctx, db.CreateClaudeSessionParams{
		ProjectID: p.ID,
		IssueID:   issueID,
		SessionID: sessionUUID,
		Title:     title,
		Alias:     alias,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			// Subagent uploads reuse the parent session.
			existing, lookupErr := s.q.GetClaudeSessionBySessionID(ctx, db.GetClaudeSessionBySessionIDParams{
				ProjectID: p.ID,
				SessionID: sessionUUID,
			})
			if lookupErr != nil {
				http.Error(w, `{"title":"Conflict","status":409,"detail":"session already exists"}`, http.StatusConflict)
				return
			}
			sessionPK = existing.ID
			sessionID = existing.SessionID
		} else {
			http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
			return
		}
	} else {
		sessionPK = sess.ID
		sessionID = sess.SessionID
	}

	// For appending to existing sessions, offset seq past existing events.
	seqOffset := int32(0)
	if sess.ID == 0 {
		cnt, cntErr := s.q.CountClaudeEvents(ctx, sessionPK)
		if cntErr == nil {
			seqOffset = int32(cnt)
		}
	}

	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		eventType := ""
		if json.Unmarshal([]byte(line), &ev) == nil {
			if t, ok := ev["type"].(string); ok {
				eventType = t
			}
		}
		_ = s.q.CreateClaudeEvent(ctx, db.CreateClaudeEventParams{
			SessionPk:        sessionPK,
			Seq:              seqOffset + int32(i),
			EventType:        eventType,
			EventJson:        []byte(line),
			AgentID:          agentID,
			IsSidechain:      isSidechain,
			AgentType:        agentType,
			AgentDescription: agentDesc,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         sessionPK,
		"session_id": sessionID,
		"events":     len(lines),
	})

	if !isSidechain {
		go s.summarizeSessionAsync(p.ID, sessionPK, lines)
	}
}

func (s *Server) summarizeSessionAsync(projectID int32, sessionPK int64, lines []string) {
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

	result, err := s.emb.complete(ctx, []llm.ChatMessage{
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

	if err := s.q.UpdateClaudeSessionSummary(ctx, db.UpdateClaudeSessionSummaryParams{
		ProjectID: projectID,
		ID:        sessionPK,
		Header:    summary.Header,
		Summary:   summary.Summary,
		Status:    summary.Status,
	}); err != nil {
		log.Printf("auto-summarize session %d: update db: %v", sessionPK, err)
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
