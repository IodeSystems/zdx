package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/danielgtaylor/huma/v2"
	"github.com/go-chi/chi/v5"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
)

type DiscussionItem struct {
	ID           int32  `json:"id"`
	ProjectID    int32  `json:"project_id"`
	Title        string `json:"title"`
	Provider     string `json:"provider"`
	Status       string `json:"status"`
	CreatedBy    string `json:"created_by"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	MessageCount int    `json:"message_count"`
}

type DiscussionMessageItem struct {
	ID           int32  `json:"id"`
	DiscussionID int32  `json:"discussion_id"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	CreatedAt    string `json:"created_at"`
}

func toDiscussionItem(d db.ZdxDiscussion) DiscussionItem {
	return DiscussionItem{
		ID:        d.ID,
		ProjectID: d.ProjectID,
		Title:     d.Title,
		Provider:  d.Provider,
		Status:    d.Status,
		CreatedBy: d.CreatedBy,
		CreatedAt: fmtTS(d.CreatedAt),
		UpdatedAt: fmtTS(d.UpdatedAt),
	}
}

func toDiscussionMessageItem(m db.ZdxDiscussionMessage) DiscussionMessageItem {
	return DiscussionMessageItem{
		ID:           m.ID,
		DiscussionID: m.DiscussionID,
		Role:         m.Role,
		Content:      m.Content,
		CreatedAt:    fmtTS(m.CreatedAt),
	}
}

func (h *Handler) registerDiscussionRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-discussions", Method: http.MethodGet, Path: "/api/dx/discussions"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Status string `query:"status"`
		}) (*struct {
			Body struct {
				Discussions []DiscussionItem `json:"discussions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Status
			if status == "" {
				status = "active"
			}
			rows, err := h.Q.ListDiscussions(ctx, db.ListDiscussionsParams{ProjectID: p.ID, Column2: status})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]DiscussionItem, len(rows))
			for i, r := range rows {
				out[i] = toDiscussionItem(r)
			}
			return &struct {
				Body struct {
					Discussions []DiscussionItem `json:"discussions"`
				}
			}{Body: struct {
				Discussions []DiscussionItem `json:"discussions"`
			}{Discussions: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-discussion", Method: http.MethodPost, Path: "/api/dx/discussions"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				Title     string `json:"title"`
				Provider  string `json:"provider,omitempty"`
				CreatedBy string `json:"created_by,omitempty"`
			}
		}) (*struct{ Body DiscussionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			provider := in.Body.Provider
			if provider == "" {
				provider = "claude"
			}
			d, err := h.Q.CreateDiscussion(ctx, db.CreateDiscussionParams{
				ProjectID: p.ID,
				Title:     in.Body.Title,
				Provider:  provider,
				CreatedBy: in.Body.CreatedBy,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body DiscussionItem }{Body: toDiscussionItem(d)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-discussion", Method: http.MethodGet, Path: "/api/dx/discussions/{id}"},
		func(ctx context.Context, in *struct {
			ID   int32  `path:"id"`
			Slug string `query:"slug" required:"true"`
		}) (*struct{ Body DiscussionItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			d, err := h.Q.GetDiscussion(ctx, db.GetDiscussionParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "discussion not found")
			}
			return &struct{ Body DiscussionItem }{Body: toDiscussionItem(d)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-discussion-status", Method: http.MethodPatch, Path: "/api/dx/discussions/{id}/status"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug   string `json:"slug"`
				Status string `json:"status"`
			}
		}) (*struct{}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			err = h.Q.UpdateDiscussionStatus(ctx, db.UpdateDiscussionStatusParams{
				ProjectID: p.ID,
				ID:        in.ID,
				Status:    in.Body.Status,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-discussion", Method: http.MethodDelete, Path: "/api/dx/discussions/{id}"},
		func(ctx context.Context, in *struct {
			ID   int32  `path:"id"`
			Slug string `query:"slug" required:"true"`
		}) (*struct{}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			err = h.Q.DeleteDiscussion(ctx, db.DeleteDiscussionParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{}{}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-discussion-messages", Method: http.MethodGet, Path: "/api/dx/discussions/{id}/messages"},
		func(ctx context.Context, in *struct {
			ID   int32  `path:"id"`
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Messages []DiscussionMessageItem `json:"messages"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			// Verify discussion belongs to project
			if _, err = h.Q.GetDiscussion(ctx, db.GetDiscussionParams{ProjectID: p.ID, ID: in.ID}); err != nil {
				return nil, apiErr(http.StatusNotFound, "discussion not found")
			}
			rows, err := h.Q.ListDiscussionMessages(ctx, in.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]DiscussionMessageItem, len(rows))
			for i, r := range rows {
				out[i] = toDiscussionMessageItem(r)
			}
			return &struct {
				Body struct {
					Messages []DiscussionMessageItem `json:"messages"`
				}
			}{Body: struct {
				Messages []DiscussionMessageItem `json:"messages"`
			}{Messages: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-discussion-message", Method: http.MethodPost, Path: "/api/dx/discussions/{id}/messages"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug    string `json:"slug"`
				Role    string `json:"role"`
				Content string `json:"content"`
			}
		}) (*struct{ Body DiscussionMessageItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if _, err = h.Q.GetDiscussion(ctx, db.GetDiscussionParams{ProjectID: p.ID, ID: in.ID}); err != nil {
				return nil, apiErr(http.StatusNotFound, "discussion not found")
			}
			m, err := h.Q.CreateDiscussionMessage(ctx, db.CreateDiscussionMessageParams{
				DiscussionID: in.ID,
				Role:         in.Body.Role,
				Content:      in.Body.Content,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body DiscussionMessageItem }{Body: toDiscussionMessageItem(m)}, nil
		})

	// Streaming LLM send: accepts a user message, persists it, calls the LLM
	// with the full message history for continuity, streams assistant deltas
	// back as SSE, and persists the final assistant turn. OpenAI-compatible
	// endpoints are stateless — continuity is supplied by sending history, not
	// by reading claude_session_id / openai_thread_id (those columns remain
	// available for a future Anthropic-native integration).
	h.Mux.Post("/api/dx/discussions/{id}/messages/send", h.handleDiscussionSendStream)
}

func (h *Handler) handleDiscussionSendStream(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	idStr := chi.URLParam(r, "id")
	id64, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.Error(w, `{"title":"Bad Request","detail":"invalid id"}`, http.StatusBadRequest)
		return
	}
	id := int32(id64)

	var body struct {
		Slug    string `json:"slug"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, `{"title":"Bad Request","detail":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if body.Slug == "" || body.Message == "" {
		http.Error(w, `{"title":"Bad Request","detail":"slug and message required"}`, http.StatusBadRequest)
		return
	}

	p, err := getProject(ctx, h.Q, body.Slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","detail":"project not found"}`, http.StatusNotFound)
		return
	}
	if _, err := h.Q.GetDiscussion(ctx, db.GetDiscussionParams{ProjectID: p.ID, ID: id}); err != nil {
		http.Error(w, `{"title":"Not Found","detail":"discussion not found"}`, http.StatusNotFound)
		return
	}

	// Persist user turn before invoking the model so partial failures still
	// leave the user message recorded.
	if _, err := h.Q.CreateDiscussionMessage(ctx, db.CreateDiscussionMessageParams{
		DiscussionID: id,
		Role:         "user",
		Content:      body.Message,
	}); err != nil {
		http.Error(w, `{"title":"Internal Server Error","detail":"persist user message"}`, http.StatusInternalServerError)
		return
	}

	history, err := h.Q.ListDiscussionMessages(ctx, id)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","detail":"load history"}`, http.StatusInternalServerError)
		return
	}
	messages := make([]llm.ChatMessage, 0, len(history))
	for _, m := range history {
		messages = append(messages, llm.ChatMessage{Role: m.Role, Content: m.Content})
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	writeSSE := func(obj any) {
		b, _ := json.Marshal(obj)
		_, _ = w.Write([]byte("data: "))
		_, _ = w.Write(b)
		_, _ = w.Write([]byte("\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}

	final, llmErr := h.Emb.StreamComplete(ctx, messages, func(delta string) {
		writeSSE(map[string]string{"delta": delta})
	})
	if llmErr != nil {
		writeSSE(map[string]string{"error": llmErr.Error()})
		return
	}

	m, err := h.Q.CreateDiscussionMessage(ctx, db.CreateDiscussionMessageParams{
		DiscussionID: id,
		Role:         "assistant",
		Content:      final,
	})
	if err != nil {
		writeSSE(map[string]string{"error": "persist assistant: " + err.Error()})
		return
	}
	writeSSE(map[string]any{"done": true, "message_id": m.ID})
}
