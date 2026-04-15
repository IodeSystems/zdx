package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/ws"
)

func (s *Server) publish(channel, eventType string, payload any) {
	s.broker.Publish(channel, ws.Message{Type: eventType, Payload: payload})
}

func (s *Server) publishIssue(slug string, issueID string, eventType string, payload any) {
	s.publish(fmt.Sprintf("project:%s:issues", slug), eventType, payload)
	s.publish(fmt.Sprintf("issue:%s", issueID), eventType, payload)
}

func (s *Server) publishTask(slug string, taskID string, eventType string, payload any) {
	s.publish(fmt.Sprintf("project:%s:tasks", slug), eventType, payload)
	s.publish(fmt.Sprintf("task:%s", taskID), eventType, payload)
}

func (s *Server) publishClaudeEvent(slug string, sessionID string, eventType string, payload any) {
	s.publish(fmt.Sprintf("project:%s:claude:%s", slug, sessionID), eventType, payload)
}

func (s *Server) publishClaudeSessionLifecycle(slug string, sessionID string, eventType string, payload any) {
	s.publish(fmt.Sprintf("project:%s:claude", slug), eventType, payload)
	s.publish(fmt.Sprintf("project:%s:claude:%s", slug, sessionID), eventType, payload)
}

func (s *Server) registerWSRoutes(api huma.API) {
	if !s.IsWSEnabled() {
		return
	}

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
		userID := ctxUserIDVal(ctx)
		if userID == 0 {
			return nil, apiErr(401, "authentication required")
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
}

func (s *Server) handleWSSubscribe(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token", http.StatusBadRequest)
		return
	}

	channel, _, err := ws.VerifyToken(s.wsSecret, token)
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
