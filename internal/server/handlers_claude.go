package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
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

	sess, err := s.q.CreateClaudeSession(ctx, db.CreateClaudeSessionParams{
		ProjectID: p.ID,
		IssueID:   issueID,
		SessionID: sessionUUID,
		Title:     title,
		Alias:     alias,
	})
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			http.Error(w, `{"title":"Conflict","status":409,"detail":"session already exists"}`, http.StatusConflict)
			return
		}
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	for seq, line := range lines {
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
			SessionPk: sess.ID,
			Seq:       int32(seq),
			EventType: eventType,
			EventJson: []byte(line),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":         sess.ID,
		"session_id": sess.SessionID,
		"events":     len(lines),
	})
}
