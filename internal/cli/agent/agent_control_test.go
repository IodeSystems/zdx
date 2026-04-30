package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// commandTestServer captures a single send-command call and lets the test
// stub the response status / body.
type commandTestServer struct {
	gotPath string
	gotBody map[string]string
	srv     *httptest.Server
}

func newCommandTestServer(t *testing.T, status int, respBody string) *commandTestServer {
	t.Helper()
	cs := &commandTestServer{}
	cs.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.gotPath = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &cs.gotBody)
		if respBody != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		if respBody != "" {
			_, _ = w.Write([]byte(respBody))
		}
	}))
	t.Cleanup(cs.srv.Close)
	return cs
}

func TestSendAgentControl_Success(t *testing.T) {
	cases := []struct {
		command string
		verb    string
		want    string
	}{
		{command: "pause", verb: "paused", want: "paused agent abc\n"},
		{command: "resume", verb: "resumed", want: "resumed agent abc\n"},
		{command: "drain", verb: "draining", want: "draining agent abc\n"},
	}
	for _, tc := range cases {
		t.Run(tc.command, func(t *testing.T) {
			cs := newCommandTestServer(t, http.StatusNoContent, "")
			c := cli.NewClient(cs.srv.URL, "")
			var out bytes.Buffer
			if err := sendAgentControl(context.Background(), &out, c, "abc", tc.command, tc.verb); err != nil {
				t.Fatalf("sendAgentControl: %v", err)
			}
			if cs.gotPath != "/api/agents/abc/command" {
				t.Errorf("path = %q, want /api/agents/abc/command", cs.gotPath)
			}
			if cs.gotBody["command"] != tc.command {
				t.Errorf("command = %q, want %q", cs.gotBody["command"], tc.command)
			}
			if out.String() != tc.want {
				t.Errorf("stdout = %q, want %q", out.String(), tc.want)
			}
		})
	}
}

func TestSendAgentControl_NotConnected(t *testing.T) {
	cs := newCommandTestServer(t, http.StatusNotFound, `{"title":"Not Found","detail":"agent not connected"}`)
	c := cli.NewClient(cs.srv.URL, "")
	var out bytes.Buffer
	err := sendAgentControl(context.Background(), &out, c, "ghost", "pause", "paused")
	if err == nil {
		t.Fatalf("expected error on 404, got nil")
	}
	if !strings.Contains(err.Error(), "ghost") || !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention agent id and 'not connected': %v", err)
	}
	if strings.Contains(err.Error(), "HTTP 404") {
		t.Errorf("404 should be rewritten as friendly hint, got: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("expected no stdout on error, got %q", out.String())
	}
}

func TestSendAgentControl_ServerError(t *testing.T) {
	cs := newCommandTestServer(t, http.StatusInternalServerError, `{"title":"Internal","detail":"boom"}`)
	c := cli.NewClient(cs.srv.URL, "")
	var out bytes.Buffer
	err := sendAgentControl(context.Background(), &out, c, "abc", "pause", "paused")
	if err == nil {
		t.Fatalf("expected error on 500, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should surface 500 status, got: %v", err)
	}
}
