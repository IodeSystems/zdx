package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/server/handlers"
	"github.com/iodesystems/zdx-go/internal/ws"
)

func (s *Server) Publish(channel, eventType string, payload any) {
	s.broker.Publish(channel, ws.Message{Type: eventType, Payload: payload})
}

func (s *Server) PublishIssue(slug string, issueID string, eventType string, payload any) {
	s.Publish(fmt.Sprintf("project:%s:issues", slug), eventType, payload)
	s.Publish(fmt.Sprintf("issue:%s", issueID), eventType, payload)
}

func (s *Server) PublishTask(slug string, taskID string, eventType string, payload any) {
	s.Publish(fmt.Sprintf("project:%s:tasks", slug), eventType, payload)
	s.Publish(fmt.Sprintf("task:%s", taskID), eventType, payload)
}

func (s *Server) PublishClaudeEvent(slug string, sessionID string, eventType string, payload any) {
	s.Publish(fmt.Sprintf("project:%s:claude:%s", slug, sessionID), eventType, payload)
}

func (s *Server) PublishClaudeSessionLifecycle(slug string, sessionID string, eventType string, payload any) {
	s.Publish(fmt.Sprintf("project:%s:claude", slug), eventType, payload)
	s.Publish(fmt.Sprintf("project:%s:claude:%s", slug, sessionID), eventType, payload)
}

// publishAgentSessionLifecycle broadcasts a provider-agnostic session
// lifecycle event (agent.session-created / agent.session-closed /
// agent.session-updated) on the per-project and per-session WS channels.
// It reuses the existing claude:* channel names so existing subscribers
// continue to receive lifecycle notifications during the transition.
func (s *Server) PublishAgentSessionLifecycle(slug string, sessionID string, eventType string, payload any) {
	s.Publish(fmt.Sprintf("project:%s:claude", slug), eventType, payload)
	s.Publish(fmt.Sprintf("project:%s:claude:%s", slug, sessionID), eventType, payload)
}

func (s *Server) registerWSRoutes(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "ws-sign",
		Method:      http.MethodPost,
		Path:        "/api/ws/sign",
		Summary:     "Sign a channel subscription request",
	}, func(ctx context.Context, in *struct {
		Body struct {
			Channel string `json:"channel" minLength:"1"`
		}
	}) (*struct {
		Body struct {
			Token string `json:"token"`
		}
	}, error) {
		userID := handlers.UserIDFromContext(ctx)
		if userID == 0 {
			return nil, handlers.APIErr(401, "authentication required")
		}
		token := ws.SignChannel(s.wsSecret, in.Body.Channel, int64(userID))
		return &struct {
			Body struct {
				Token string `json:"token"`
			}
		}{Body: struct {
			Token string `json:"token"`
		}{Token: token}}, nil
	})

	s.mux.HandleFunc("/api/ws/subscribe", s.handleWSSubscribe)

	s.registerAdminWSDiagnostics(api)
}

// registerAdminWSDiagnostics exposes admin-only endpoints for inspecting the
// live WS client registry and publishing test echo messages. Routes live under
// /api/admin/ so they inherit the admin-role middleware.
func (s *Server) registerAdminWSDiagnostics(api huma.API) {
	type ClientItem struct {
		ID          int64  `json:"id"`
		Channel     string `json:"channel"`
		UserID      int64  `json:"user_id"`
		RemoteAddr  string `json:"remote_addr"`
		ConnectedAt string `json:"connected_at"`
	}

	huma.Register(api, huma.Operation{
		OperationID: "list-ws-clients",
		Method:      http.MethodGet,
		Path:        "/api/admin/ws/clients",
		Summary:     "List live WebSocket subscribers",
	}, func(ctx context.Context, _ *struct{}) (*struct {
		Body struct {
			Clients []ClientItem `json:"clients"`
		}
	}, error) {
		s.wsClientsMu.Lock()
		items := make([]ClientItem, 0, len(s.wsClients))
		for _, c := range s.wsClients {
			items = append(items, ClientItem{
				ID:          c.ID,
				Channel:     c.Channel,
				UserID:      c.UserID,
				RemoteAddr:  c.RemoteAddr,
				ConnectedAt: c.ConnectedAt.Format(time.RFC3339),
			})
		}
		s.wsClientsMu.Unlock()
		sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
		return &struct {
			Body struct {
				Clients []ClientItem `json:"clients"`
			}
		}{Body: struct {
			Clients []ClientItem `json:"clients"`
		}{Clients: items}}, nil
	})

	huma.Register(api, huma.Operation{
		OperationID: "ws-echo",
		Method:      http.MethodPost,
		Path:        "/api/admin/ws/echo",
		Summary:     "Publish an admin.echo message to a channel",
	}, func(ctx context.Context, in *struct {
		Body struct {
			Channel string `json:"channel" minLength:"1"`
			Payload string `json:"payload,omitempty"`
		}
	}) (*struct {
		Body struct {
			SentAt string `json:"sent_at"`
		}
	}, error) {
		sentAt := time.Now().UTC()
		s.Publish(in.Body.Channel, "admin.echo", map[string]any{
			"payload": in.Body.Payload,
			"sent_at": sentAt.Format(time.RFC3339Nano),
		})
		return &struct {
			Body struct {
				SentAt string `json:"sent_at"`
			}
		}{Body: struct {
			SentAt string `json:"sent_at"`
		}{SentAt: sentAt.Format(time.RFC3339Nano)}}, nil
	})
}

func (s *Server) registerWSClient(channel string, userID int64, remoteAddr string) int64 {
	s.wsClientsMu.Lock()
	defer s.wsClientsMu.Unlock()
	s.wsClientSeq++
	id := s.wsClientSeq
	s.wsClients[id] = &wsClientEntry{
		ID:          id,
		Channel:     channel,
		UserID:      userID,
		RemoteAddr:  remoteAddr,
		ConnectedAt: time.Now().UTC(),
	}
	return id
}

func (s *Server) unregisterWSClient(id int64) {
	s.wsClientsMu.Lock()
	delete(s.wsClients, id)
	s.wsClientsMu.Unlock()
}

func (s *Server) handleWSSubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	channel, userID, err := ws.VerifyToken(s.wsSecret, token)
	if err != nil {
		http.Error(w, "invalid token: "+err.Error(), http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws: accept error: %v", err)
		return
	}
	defer conn.CloseNow()

	clientID := s.registerWSClient(channel, userID, r.RemoteAddr)
	defer s.unregisterWSClient(clientID)

	sub := s.broker.Subscribe(channel)
	defer sub.Close()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			conn.Close(websocket.StatusNormalClosure, "")
			return
		case msg, ok := <-sub.C:
			if !ok {
				conn.Close(websocket.StatusNormalClosure, "")
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				continue
			}
			if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
				return
			}
		}
	}
}
