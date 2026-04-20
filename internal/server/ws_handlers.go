package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"sync"
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

	// Single multiplexed websocket endpoint. Lives outside /api/ so it bypasses
	// the API-key middleware — auth happens inside the protocol via signed
	// subscribe tokens.
	s.mux.HandleFunc("/ws", s.handleWS)

	s.registerAdminWSDiagnostics(api)
}

// registerAdminWSDiagnostics exposes admin-only endpoints for inspecting the
// live WS client registry and publishing test echo messages. Routes live under
// /api/admin/ so they inherit the admin-role middleware.
func (s *Server) registerAdminWSDiagnostics(api huma.API) {
	type ClientItem struct {
		ID          int64    `json:"id"`
		Channels    []string `json:"channels"`
		UserID      int64    `json:"user_id"`
		RemoteAddr  string   `json:"remote_addr"`
		ConnectedAt string   `json:"connected_at"`
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
			channels := append([]string(nil), c.Channels...)
			sort.Strings(channels)
			items = append(items, ClientItem{
				ID:          c.ID,
				Channels:    channels,
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

func (s *Server) registerWSClient(userID int64, remoteAddr string) int64 {
	s.wsClientsMu.Lock()
	defer s.wsClientsMu.Unlock()
	s.wsClientSeq++
	id := s.wsClientSeq
	s.wsClients[id] = &wsClientEntry{
		ID:          id,
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

func (s *Server) updateWSClientChannels(id int64, channels []string) {
	s.wsClientsMu.Lock()
	if c, ok := s.wsClients[id]; ok {
		c.Channels = channels
	}
	s.wsClientsMu.Unlock()
}

// clientMsg is the incoming JSON envelope on the multiplexed /ws connection.
type clientMsg struct {
	Type    string `json:"type"`
	Token   string `json:"token,omitempty"`
	Channel string `json:"channel,omitempty"`
}

// serverMsg is the outgoing JSON envelope to the websocket client.
type serverMsg struct {
	Type    string `json:"type"`
	Channel string `json:"channel,omitempty"`
	Event   string `json:"event,omitempty"`
	Payload any    `json:"payload,omitempty"`
	Message string `json:"message,omitempty"`
}

// handleWS is the single multiplexed websocket endpoint at /ws. Clients open
// one persistent connection, then send {"type":"subscribe","token":...} messages
// to attach to broker channels. Each channel subscription is relayed over the
// same connection as {"type":"message","channel":...,"event":...,"payload":...}.
func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Printf("ws: accept error: %v", err)
		return
	}
	defer conn.CloseNow()

	clientID := s.registerWSClient(0, r.RemoteAddr)
	defer s.unregisterWSClient(clientID)

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Serialize writes to the single websocket connection across the read
	// loop and every per-subscription relay goroutine.
	var writeMu sync.Mutex
	writeJSON := func(msg serverMsg) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		return conn.Write(ctx, websocket.MessageText, data)
	}

	// Active channel subscriptions for this client.
	var subsMu sync.Mutex
	subs := make(map[string]*ws.Subscription)
	var userID int64

	snapshotChannels := func() []string {
		subsMu.Lock()
		defer subsMu.Unlock()
		out := make([]string, 0, len(subs))
		for ch := range subs {
			out = append(out, ch)
		}
		return out
	}

	closeAllSubs := func() {
		subsMu.Lock()
		defer subsMu.Unlock()
		for _, sub := range subs {
			sub.Close()
		}
		subs = map[string]*ws.Subscription{}
	}
	defer closeAllSubs()

	for {
		msgType, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		if msgType != websocket.MessageText {
			continue
		}

		var in clientMsg
		if err := json.Unmarshal(data, &in); err != nil {
			_ = writeJSON(serverMsg{Type: "error", Message: "malformed message"})
			continue
		}

		switch in.Type {
		case "subscribe":
			if in.Token == "" {
				_ = writeJSON(serverMsg{Type: "error", Message: "missing token"})
				continue
			}
			channel, tokUserID, verr := ws.VerifyToken(s.wsSecret, in.Token)
			if verr != nil {
				_ = writeJSON(serverMsg{Type: "error", Message: "invalid token: " + verr.Error()})
				continue
			}
			// First successful subscribe defines the user for this connection.
			if userID == 0 {
				userID = tokUserID
				s.wsClientsMu.Lock()
				if c, ok := s.wsClients[clientID]; ok {
					c.UserID = tokUserID
				}
				s.wsClientsMu.Unlock()
			}

			subsMu.Lock()
			if _, already := subs[channel]; already {
				subsMu.Unlock()
				_ = writeJSON(serverMsg{Type: "subscribed", Channel: channel})
				continue
			}
			sub := s.broker.Subscribe(channel)
			subs[channel] = sub
			subsMu.Unlock()

			s.updateWSClientChannels(clientID, snapshotChannels())
			_ = writeJSON(serverMsg{Type: "subscribed", Channel: channel})

			go func(ch string, sub *ws.Subscription) {
				for {
					select {
					case <-ctx.Done():
						return
					case msg, ok := <-sub.C:
						if !ok {
							return
						}
						if err := writeJSON(serverMsg{
							Type:    "message",
							Channel: ch,
							Event:   msg.Type,
							Payload: msg.Payload,
						}); err != nil {
							return
						}
					}
				}
			}(channel, sub)

		case "unsubscribe":
			if in.Channel == "" {
				_ = writeJSON(serverMsg{Type: "error", Message: "missing channel"})
				continue
			}
			subsMu.Lock()
			sub, ok := subs[in.Channel]
			if ok {
				delete(subs, in.Channel)
			}
			subsMu.Unlock()
			if ok {
				sub.Close()
			}
			s.updateWSClientChannels(clientID, snapshotChannels())
			_ = writeJSON(serverMsg{Type: "unsubscribed", Channel: in.Channel})

		case "ping":
			_ = writeJSON(serverMsg{Type: "pong"})

		default:
			_ = writeJSON(serverMsg{Type: "error", Message: "unknown type: " + in.Type})
		}
	}
}
