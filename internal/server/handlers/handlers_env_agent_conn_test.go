package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"nhooyr.io/websocket"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/envagentconn"
)

// fakeEnvAgentStore records every upsert so tests can assert content +
// invocation count without spinning up a real Postgres.
type fakeEnvAgentStore struct {
	mu      sync.Mutex
	project db.ZdxProject
	env     db.GetEnvironmentRow
	rows    []db.UpsertEnvAgentHeartbeatParams
	// projectErr / envErr let a test simulate a missing project/env without
	// having to register a separate negative-path store implementation.
	projectErr error
	envErr     error
}

func (f *fakeEnvAgentStore) GetProjectBySlug(_ context.Context, _ string) (db.ZdxProject, error) {
	if f.projectErr != nil {
		return db.ZdxProject{}, f.projectErr
	}
	return f.project, nil
}

func (f *fakeEnvAgentStore) GetEnvironment(_ context.Context, _ db.GetEnvironmentParams) (db.GetEnvironmentRow, error) {
	if f.envErr != nil {
		return db.GetEnvironmentRow{}, f.envErr
	}
	return f.env, nil
}

func (f *fakeEnvAgentStore) UpsertEnvAgentHeartbeat(_ context.Context, arg db.UpsertEnvAgentHeartbeatParams) (db.ZdxEnvAgent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rows = append(f.rows, arg)
	return db.ZdxEnvAgent{
		ID:             int32(len(f.rows)),
		EnvID:          arg.EnvID,
		AgentID:        arg.AgentID,
		Hostname:       arg.Hostname,
		Version:        arg.Version,
		Os:             arg.Os,
		DeployedCommit: arg.DeployedCommit,
		UptimeSecs:     arg.UptimeSecs,
	}, nil
}

func (f *fakeEnvAgentStore) upsertCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.rows)
}

func (f *fakeEnvAgentStore) lastUpsert() db.UpsertEnvAgentHeartbeatParams {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.rows) == 0 {
		return db.UpsertEnvAgentHeartbeatParams{}
	}
	return f.rows[len(f.rows)-1]
}

// buildEnvAgentE2EServer wires the WS handler onto a chi mux behind an
// httptest.Server. Mirrors buildE2EServer in handlers_agents_command_e2e_test.go.
// role can be "admin" (allowed) or "" to drive the unauthenticated rejection
// path.
func buildEnvAgentE2EServer(t *testing.T, role string, store EnvAgentStore) (*httptest.Server, *envagentconn.Registry) {
	t.Helper()
	mux := chi.NewMux()
	h := &Handler{Deps: &Deps{}}
	reg := envagentconn.New()
	connectHandler := h.HandleEnvAgentConnect(reg, store)
	mux.Get("/api/dx/env-agents/connect", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if role != "" {
			ctx = context.WithValue(ctx, CtxUserRole, role)
		}
		connectHandler(w, r.WithContext(ctx))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, reg
}

func wsDial(t *testing.T, urlStr string) (*websocket.Conn, *http.Response) {
	t.Helper()
	wsURL := strings.Replace(urlStr, "http://", "ws://", 1) + "/api/dx/env-agents/connect"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, resp, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil && resp == nil {
		t.Fatalf("ws dial: %v", err)
	}
	return c, resp
}

// readJSON reads one JSON frame from c.
func readJSON(t *testing.T, c *websocket.Conn, out any) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, data, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("ws read: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("ws decode: %v (raw=%s)", err, data)
	}
}

func writeJSON(t *testing.T, c *websocket.Conn, v any) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := c.Write(ctx, websocket.MessageText, data); err != nil {
		t.Fatalf("ws write: %v", err)
	}
}

func TestEnvAgentConnect_HandshakeUpsertsRow(t *testing.T) {
	store := &fakeEnvAgentStore{
		project: db.ZdxProject{ID: 7, Slug: "myproj"},
		env:     db.GetEnvironmentRow{ID: 42, ProjectID: 7, Name: "prod"},
	}
	srv, reg := buildEnvAgentE2EServer(t, "admin", store)

	c, _ := wsDial(t, srv.URL)
	defer c.Close(websocket.StatusNormalClosure, "") //nolint:errcheck

	writeJSON(t, c, EnvAgentRegisterMsg{
		AgentID:        "envd-1",
		ProjectSlug:    "myproj",
		EnvName:        "prod",
		Hostname:       "host-a",
		Version:        "v1.2.3",
		OS:             "linux",
		DeployedCommit: "abc123",
		UptimeSecs:     60,
	})

	var ack map[string]any
	readJSON(t, c, &ack)
	if ack["type"] != "registered" {
		t.Fatalf("ack type = %v, want registered", ack["type"])
	}

	if store.upsertCount() != 1 {
		t.Fatalf("upsert count = %d, want 1", store.upsertCount())
	}
	got := store.lastUpsert()
	if got.EnvID != 42 || got.AgentID != "envd-1" || got.Version != "v1.2.3" || got.Os != "linux" || got.DeployedCommit != "abc123" || got.UptimeSecs != 60 {
		t.Errorf("upsert params: %+v", got)
	}

	if !pollUntil(time.Second, func() bool {
		_, ok := reg.Get("envd-1")
		return ok
	}) {
		t.Fatal("agent not registered with registry")
	}
}

func TestEnvAgentConnect_HeartbeatUpdatesRow(t *testing.T) {
	store := &fakeEnvAgentStore{
		project: db.ZdxProject{ID: 7, Slug: "myproj"},
		env:     db.GetEnvironmentRow{ID: 42, ProjectID: 7, Name: "prod"},
	}
	srv, _ := buildEnvAgentE2EServer(t, "admin", store)

	c, _ := wsDial(t, srv.URL)
	defer c.Close(websocket.StatusNormalClosure, "") //nolint:errcheck

	writeJSON(t, c, EnvAgentRegisterMsg{
		AgentID: "envd-1", ProjectSlug: "myproj", EnvName: "prod",
		Version: "v1.0.0", UptimeSecs: 10,
	})
	var ack map[string]any
	readJSON(t, c, &ack)

	writeJSON(t, c, EnvAgentHeartbeatMsg{
		Type: "heartbeat", Version: "v1.0.1", OS: "linux", DeployedCommit: "def456", UptimeSecs: 120,
	})

	if !pollUntil(time.Second, func() bool { return store.upsertCount() == 2 }) {
		t.Fatalf("upsert count = %d, want 2", store.upsertCount())
	}
	got := store.lastUpsert()
	if got.Version != "v1.0.1" || got.DeployedCommit != "def456" || got.UptimeSecs != 120 {
		t.Errorf("heartbeat upsert params: %+v", got)
	}
}

func TestEnvAgentConnect_NonAdminRejected(t *testing.T) {
	srv, _ := buildEnvAgentE2EServer(t, "user", nil)

	wsURL := strings.Replace(srv.URL, "http://", "ws://", 1) + "/api/dx/env-agents/connect"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		// Some WS clients surface the close as a dial error — acceptable.
		return
	}
	defer c.Close(websocket.StatusInternalError, "") //nolint:errcheck

	// Server-side close should arrive as the next Read.
	_, _, readErr := c.Read(ctx)
	if readErr == nil {
		t.Fatal("expected read error from server-side close, got nil")
	}
	if status := websocket.CloseStatus(readErr); status != websocket.StatusPolicyViolation {
		t.Errorf("close status = %v, want %v", status, websocket.StatusPolicyViolation)
	}
}

func TestEnvAgentConnect_InvalidHandshakeRejected(t *testing.T) {
	store := &fakeEnvAgentStore{}
	srv, _ := buildEnvAgentE2EServer(t, "admin", store)

	c, _ := wsDial(t, srv.URL)
	defer c.Close(websocket.StatusInternalError, "") //nolint:errcheck

	// Empty handshake — missing required fields.
	writeJSON(t, c, EnvAgentRegisterMsg{AgentID: "envd-1"})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, _, err := c.Read(ctx)
	if err == nil {
		t.Fatal("expected read error from server-side close, got nil")
	}
	if status := websocket.CloseStatus(err); status != websocket.StatusProtocolError {
		t.Errorf("close status = %v, want %v", status, websocket.StatusProtocolError)
	}
	if store.upsertCount() != 0 {
		t.Errorf("upsert fired %d times on invalid handshake; want 0", store.upsertCount())
	}
}
