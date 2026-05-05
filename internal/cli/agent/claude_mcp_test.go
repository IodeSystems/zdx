package agent

import (
	"encoding/json"
	"testing"
)

// buildClaudeMCPConfig is the inline JSON claude consumes via --mcp-config.
// Shape:
//
//	{"mcpServers":{"dx-tools":{"command":<argv0>,"args":[<argv[1:]>]}}}
//
// The shape is part of claude's CLI contract; format drift here would
// silently disable tool dispatch in dev-container mode.
func TestBuildClaudeMCPConfig_DockerExecArgv(t *testing.T) {
	argv := []string{"docker", "exec", "-i", "slot-7", "dx-agent", "--mcp-stdio"}
	got, err := buildClaudeMCPConfig(argv)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, got)
	}
	srv, ok := parsed.MCPServers["dx-tools"]
	if !ok {
		t.Fatalf("dx-tools server missing: %s", got)
	}
	if srv.Command != "docker" {
		t.Errorf("command = %q, want docker", srv.Command)
	}
	wantArgs := []string{"exec", "-i", "slot-7", "dx-agent", "--mcp-stdio"}
	if len(srv.Args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d: %v", len(srv.Args), len(wantArgs), srv.Args)
	}
	for i, w := range wantArgs {
		if srv.Args[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, srv.Args[i], w)
		}
	}
}

func TestBuildClaudeMCPConfig_SingleCommandHasEmptyArgs(t *testing.T) {
	got, err := buildClaudeMCPConfig([]string{"dx-agent"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var parsed struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got := parsed.MCPServers["dx-tools"].Command; got != "dx-agent" {
		t.Errorf("command = %q, want dx-agent", got)
	}
	if len(parsed.MCPServers["dx-tools"].Args) != 0 {
		t.Errorf("args should be empty for single-command argv: %v", parsed.MCPServers["dx-tools"].Args)
	}
}

func TestBuildClaudeMCPConfig_EmptyArgvErrors(t *testing.T) {
	_, err := buildClaudeMCPConfig(nil)
	if err == nil {
		t.Fatal("expected error on empty argv, got nil")
	}
}
