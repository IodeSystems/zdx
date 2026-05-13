package agent

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// renderSessionEvent tests run with ANSI disabled so assertions stay readable.
// colorsEnabled is package-level and already false under `go test` (stdout is
// not a char device), but we force it to be safe.
func forceNoColor(t *testing.T) {
	t.Helper()
	prev := colorsEnabled
	colorsEnabled = false
	t.Cleanup(func() { colorsEnabled = prev })
}

func TestRenderSessionEvent(t *testing.T) {
	forceNoColor(t)

	cases := []struct {
		name string
		line string
		want string
	}{
		{
			name: "queue-operation is silent",
			line: `{"type":"queue-operation","operation":"enqueue"}`,
			want: "",
		},
		{
			name: "attachment is silent",
			line: `{"type":"attachment"}`,
			want: "",
		},
		{
			name: "ai-title",
			line: `{"type":"ai-title","title":"Refactor renderer"}`,
			want: "● Refactor renderer",
		},
		{
			name: "assistant text trimmed+flattened",
			line: `{"type":"assistant","message":{"content":[{"type":"text","text":"  hello\nworld  "}]}}`,
			want: "hello world",
		},
		{
			name: "assistant thinking",
			line: `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"planning next step"}]}}`,
			want: "⟡ planning next step",
		},
		{
			name: "assistant tool_use Bash",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"x","name":"Bash","input":{"command":"ls -la"}}]}}`,
			want: "[Bash] ls -la",
		},
		{
			name: "assistant tool_use Read",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"x","name":"Read","input":{"file_path":"/tmp/x"}}]}}`,
			want: "[Read] /tmp/x",
		},
		{
			name: "assistant multi-block emits newline-separated lines",
			line: `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"think"},{"type":"tool_use","id":"x","name":"Grep","input":{"pattern":"foo"}}]}}`,
			want: "⟡ think\n[Grep] foo",
		},
		{
			name: "user tool_result ok with string content",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"x","is_error":false,"content":"matched 3 files"}]}}`,
			want: "✓ tool: matched 3 files",
		},
		{
			name: "user tool_result err with array content",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"x","is_error":true,"content":[{"type":"text","text":"boom"}]}]}}`,
			want: "✗ tool: boom",
		},
		{
			name: "user plain-string content is silent",
			line: `{"type":"user","message":{"content":"hello"}}`,
			want: "",
		},
		{
			name: "unknown event type is silent",
			line: `{"type":"something-else"}`,
			want: "",
		},
		{
			name: "malformed json is silent",
			line: `not-json`,
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			toolNames := map[string]string{}
			got := renderSessionEvent([]byte(tc.line), toolNames)
			if got != tc.want {
				t.Fatalf("line=%s\n  got  %q\n  want %q", tc.line, got, tc.want)
			}
		})
	}
}

// tool_result should show the tool name that was captured from the preceding
// tool_use via toolNames[id].
func TestRenderSessionEvent_ToolNameLink(t *testing.T) {
	forceNoColor(t)
	toolNames := map[string]string{}

	// 1) record tool_use name under its id
	_ = renderSessionEvent([]byte(`{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Bash","input":{"command":"ls"}}]}}`), toolNames)
	if toolNames["toolu_1"] != "Bash" {
		t.Fatalf("expected toolNames[toolu_1]=Bash, got %v", toolNames)
	}

	// 2) matching tool_result should label as Bash (not fallback "tool")
	got := renderSessionEvent([]byte(`{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","is_error":false,"content":"ok"}]}}`), toolNames)
	if got != "✓ Bash: ok" {
		t.Fatalf("want '✓ Bash: ok', got %q", got)
	}
}

// flattenTrunc should strip newlines, trim whitespace, and truncate with …
// at the rune (not byte) boundary.
func TestFlattenTrunc(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"", 10, ""},
		{"   ", 10, ""},
		{"hello", 10, "hello"},
		{"hello\nworld", 20, "hello world"},
		{"abcdefghij", 5, "abcde…"},
		{"日本語テスト文字", 3, "日本語…"},
	}
	for _, tc := range cases {
		if got := flattenTrunc(tc.in, tc.n); got != tc.want {
			t.Errorf("flattenTrunc(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

// colorize is a no-op when colors are disabled and wraps with reset otherwise.
func TestColorize(t *testing.T) {
	prev := colorsEnabled

	colorsEnabled = false
	if got := colorize(ansiRed, "x"); got != "x" {
		t.Errorf("disabled: want 'x', got %q", got)
	}

	colorsEnabled = true
	got := colorize(ansiRed, "x")
	if !strings.HasPrefix(got, ansiRed) || !strings.HasSuffix(got, ansiReset) {
		t.Errorf("enabled: want wrapped with red..reset, got %q", got)
	}

	colorsEnabled = prev
}

func TestBuildClaudeEnv(t *testing.T) {
	base := []string{"PATH=/usr/bin", "HOME=/h"}

	got := buildClaudeEnv(base, "sid-1", "agent-A", "", false, "", "")
	wantContains := []string{"PATH=/usr/bin", "HOME=/h", "ZDX_SESSION_ID=sid-1", "ZDX_AGENT_ID=agent-A", "DX_AUTHOR_ALIAS=agent-A"}
	for _, kv := range wantContains {
		if !contains(got, kv) {
			t.Errorf("non-srcless env missing %q in %v", kv, got)
		}
	}
	if contains(got, "DX_GLOBAL=1") {
		t.Errorf("non-srcless env should not contain DX_GLOBAL=1: %v", got)
	}
	for _, kv := range got {
		if strings.HasPrefix(kv, "ZDX_TRACE_ID=") {
			t.Errorf("empty traceID should not produce ZDX_TRACE_ID env: %q", kv)
		}
	}

	got = buildClaudeEnv(base, "sid-2", "agent-B", "trace-xyz", true, "", "")
	if !contains(got, "DX_GLOBAL=1") {
		t.Errorf("srcless env missing DX_GLOBAL=1: %v", got)
	}
	if !contains(got, "ZDX_TRACE_ID=trace-xyz") {
		t.Errorf("ZDX_TRACE_ID not exported when traceID set: %v", got)
	}
	for _, kv := range []string{"ZDX_SESSION_ID=sid-2", "ZDX_AGENT_ID=agent-B", "DX_AUTHOR_ALIAS=agent-B"} {
		if !contains(got, kv) {
			t.Errorf("srcless env missing %q in %v", kv, got)
		}
	}

	// Admin token in base must be replaced by the scoped token (never two DX_REMOTE_API_KEY entries).
	baseWithAdmin := []string{"DX_REMOTE_API_KEY=admin-token-xxx", "PATH=/usr/bin"}
	got = buildClaudeEnv(baseWithAdmin, "sid-3", "agent-C", "", false, "scoped-token-yyy", "")
	if !contains(got, "DX_REMOTE_API_KEY=scoped-token-yyy") {
		t.Errorf("scoped token not injected: %v", got)
	}
	if contains(got, "DX_REMOTE_API_KEY=admin-token-xxx") {
		t.Errorf("admin token must be stripped from env: %v", got)
	}
	count := 0
	for _, kv := range got {
		if strings.HasPrefix(kv, "DX_REMOTE_API_KEY=") {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected exactly 1 DX_REMOTE_API_KEY entry, got %d: %v", count, got)
	}

	// DX_AGENT_ISSUE injection.
	got = buildClaudeEnv(base, "sid-4", "agent-D", "", false, "", "IS-123")
	if !contains(got, "DX_AGENT_ISSUE=IS-123") {
		t.Errorf("DX_AGENT_ISSUE not injected: %v", got)
	}

	// Empty issueID should not inject DX_AGENT_ISSUE.
	got = buildClaudeEnv(base, "sid-5", "agent-E", "", false, "", "")
	for _, kv := range got {
		if strings.HasPrefix(kv, "DX_AGENT_ISSUE=") {
			t.Errorf("empty issueID should not produce DX_AGENT_ISSUE env: %q", kv)
		}
	}
}

func contains(s []string, needle string) bool {
	for _, v := range s {
		if v == needle {
			return true
		}
	}
	return false
}

func projectsHandler(projects []map[string]any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"projects": projects})
	}
}

func TestFetchProjectVision(t *testing.T) {
	title := "My App"
	desc := "Does great things"

	cases := []struct {
		name    string
		handler http.HandlerFunc
		rc      func(url string) remoteConfig
		want    string
	}{
		{
			name:    "title and description joined",
			handler: projectsHandler([]map[string]any{{"slug": "my-app", "title": &title, "description": &desc}}),
			rc:      func(url string) remoteConfig { return remoteConfig{url: url, slug: "my-app", key: "k"} },
			want:    "My App — Does great things",
		},
		{
			name:    "title only when description nil",
			handler: projectsHandler([]map[string]any{{"slug": "my-app", "title": &title, "description": nil}}),
			rc:      func(url string) remoteConfig { return remoteConfig{url: url, slug: "my-app", key: "k"} },
			want:    "My App",
		},
		{
			name:    "empty string when slug not found",
			handler: projectsHandler([]map[string]any{{"slug": "other", "title": &title, "description": &desc}}),
			rc:      func(url string) remoteConfig { return remoteConfig{url: url, slug: "my-app", key: "k"} },
			want:    "",
		},
		{
			name: "empty string on non-200",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			},
			rc:   func(url string) remoteConfig { return remoteConfig{url: url, slug: "my-app", key: "k"} },
			want: "",
		},
		{
			name:    "empty string when remoteConfig invalid",
			handler: projectsHandler(nil),
			rc:      func(_ string) remoteConfig { return remoteConfig{} },
			want:    "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()
			rc := tc.rc(srv.URL)
			got := fetchProjectVision(rc)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestBuildSessionPrompt(t *testing.T) {
	todo := &claimedTodo{ID: 42, Kind: "dev", TargetType: "task", TargetID: "TK-1", Text: "Do the thing"}

	cases := []struct {
		name    string
		vision  string
		issueID string
		persona string
		todo    *claimedTodo
		check   func(t *testing.T, got string)
	}{
		{
			name:   "vision included",
			vision: "Great platform",
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Project vision: Great platform\n\n") {
					t.Fatalf("missing vision block, got %q", got)
				}
			},
		},
		{
			name:   "empty vision omits prefix",
			vision: "",
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "Project vision") {
					t.Fatalf("unexpected vision prefix, got %q", got)
				}
			},
		},
		{
			name:   "todo text follows vision",
			vision: "V",
			todo:   todo,
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Project vision: V\n\n") {
					t.Fatalf("missing vision block, got %q", got)
				}
				if !strings.Contains(got, "Claimed todo 42 [dev] target=task:TK-1") {
					t.Fatalf("missing todo header, got %q", got)
				}
				if !strings.Contains(got, "Do the thing") {
					t.Fatalf("missing todo text, got %q", got)
				}
			},
		},
		{
			name:    "issue fallback without todo",
			vision:  "",
			issueID: "IS-99",
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Work on issue IS-99") {
					t.Fatalf("missing issue prompt, got %q", got)
				}
			},
		},
		{
			name:   "default persona injects dev block",
			vision: "V",
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Persona: dev") {
					t.Fatalf("default persona prompt missing, got %q", got)
				}
			},
		},
		{
			name:    "explicit persona swaps block",
			vision:  "V",
			persona: "reviewer",
			check: func(t *testing.T, got string) {
				if !strings.Contains(got, "Persona: reviewer") {
					t.Fatalf("reviewer persona prompt missing, got %q", got)
				}
				if strings.Contains(got, "Persona: dev") {
					t.Fatalf("reviewer persona should replace dev, got %q", got)
				}
			},
		},
		{
			name:    "todo takes precedence over issueID",
			vision:  "",
			issueID: "IS-99",
			todo:    todo,
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "Work on issue") {
					t.Fatalf("todo should win over issueID, got %q", got)
				}
				if !strings.Contains(got, "Claimed todo 42") {
					t.Fatalf("missing todo header, got %q", got)
				}
			},
		},
		{
			name: "todo prompt embeds incomplete-report protocol",
			todo: todo,
			check: func(t *testing.T, got string) {
				wantSubstrings := []string{
					"INCOMPLETE-REPORT PROTOCOL",
					"dx todo incomplete TK-1",
					"--reason=",
					"--explanation=",
					"capability_gap",
					"ambiguous_spec",
					"external_dep",
					"needs_decision",
					"permission_denied",
					"environment_broken",
					"preexisting_test_failure",
					"flaky_test",
					"protocol violation",
				}
				for _, s := range wantSubstrings {
					if !strings.Contains(got, s) {
						t.Fatalf("prompt missing %q\n--- prompt ---\n%s", s, got)
					}
				}
			},
		},
		{
			name:    "issue-only prompt does not embed incomplete-report protocol",
			issueID: "IS-99",
			check: func(t *testing.T, got string) {
				if strings.Contains(got, "INCOMPLETE-REPORT PROTOCOL") {
					t.Fatalf("issue-only prompt must not include todo-only protocol section, got %q", got)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := buildSessionPrompt(tc.vision, tc.issueID, tc.persona, tc.todo)
			tc.check(t, got)
		})
	}
}

func TestInjectMCPDockerExecEnv(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		kv   map[string]string
		want []string // exact expected output
	}{
		{
			name: "non-docker passthrough",
			argv: []string{"/usr/local/bin/dx-agent", "--mcp-stdio"},
			kv:   map[string]string{"FOO": "bar"},
			want: []string{"/usr/local/bin/dx-agent", "--mcp-stdio"},
		},
		{
			name: "empty kv passthrough",
			argv: []string{"docker", "exec", "-i", "slot", "/workspace/bin/dx-agent", "--mcp-stdio"},
			kv:   nil,
			want: []string{"docker", "exec", "-i", "slot", "/workspace/bin/dx-agent", "--mcp-stdio"},
		},
		{
			name: "single var injected after -i, before container name",
			argv: []string{"docker", "exec", "-i", "slot", "/workspace/bin/dx-agent", "--mcp-stdio"},
			kv:   map[string]string{"ZDX_TRACE_ID": "trace-abc"},
			want: []string{"docker", "exec", "-i", "-e", "ZDX_TRACE_ID=trace-abc", "slot", "/workspace/bin/dx-agent", "--mcp-stdio"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := injectMCPDockerExecEnv(tc.argv, tc.kv)
			if len(got) != len(tc.want) {
				t.Fatalf("len: want %d got %d\nwant=%v\ngot=%v", len(tc.want), len(got), tc.want, got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("argv[%d]: want %q got %q", i, tc.want[i], got[i])
				}
			}
		})
	}
}

func TestInjectMCPDockerExecEnvMultipleVarsTokenCount(t *testing.T) {
	// Map iteration order is unstable so we check the token count + presence
	// rather than exact ordering.
	argv := []string{"docker", "exec", "-i", "slot", "dx-agent", "--mcp-stdio"}
	kv := map[string]string{"ZDX_TRACE_ID": "trace-abc", "ZDX_AGENT_ID": "smoke-0"}
	got := injectMCPDockerExecEnv(argv, kv)
	// Each kv adds 2 tokens (-e + KEY=VAL).
	if len(got) != len(argv)+2*len(kv) {
		t.Errorf("token count: want %d got %d (argv=%v)", len(argv)+2*len(kv), len(got), got)
	}
	joined := strings.Join(got, " ")
	for k, v := range kv {
		if !strings.Contains(joined, "-e "+k+"="+v) {
			t.Errorf("missing -e %s=%s in %s", k, v, joined)
		}
	}
}

func TestMcpDockerExecEnv(t *testing.T) {
	got := mcpDockerExecEnv("sid-1", "smoke-0", "trace-xyz", true)
	for _, k := range []string{"ZDX_SESSION_ID", "ZDX_AGENT_ID", "DX_AUTHOR_ALIAS", "ZDX_TRACE_ID", "DX_GLOBAL"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q: %v", k, got)
		}
	}
	got = mcpDockerExecEnv("", "", "", false)
	if len(got) != 0 {
		t.Errorf("all-empty should produce no kv: %v", got)
	}
}
