package agent

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func agentClaudeCmd() *cobra.Command {
	var loop bool
	var alias string
	var issue string
	var chrome bool
	cmd := &cobra.Command{
		Use:   "claude",
		Short: "Run Claude agent sessions with zdx integration",
		Long:  "Launch Claude CLI sessions with automatic session streaming, subagent discovery, and token usage tracking.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.Load()
			rc := remoteConfig{
				url:  cfg.RemoteURL(),
				slug: cfg.RemoteSlug(),
				key:  config.RemoteAPIKey(),
			}

			if loop {
				return runLoop(rc, alias, chrome)
			}
			ctx, cancel := context.WithCancel(cmd.Context())
			defer cancel()
			installReleaseOnSignal(rc, alias, "", nil, cancel)
			sid := uuid.New().String()
			return runSession(ctx, rc, sid, issue, alias, chrome, "", false)
		},
	}
	cmd.Flags().BoolVar(&loop, "loop", false, "loop: pick work via solo, run sessions, repeat")
	cmd.Flags().StringVar(&alias, "alias", "", "agent alias for identification")
	cmd.Flags().StringVar(&issue, "issue", "", "issue to work on (single session mode)")
	cmd.Flags().BoolVar(&chrome, "chrome", true, "pass --chrome to claude CLI")
	return cmd
}

type remoteConfig struct {
	url  string
	slug string
	key  string
}

func (r remoteConfig) valid() bool {
	return r.url != "" && r.slug != "" && r.key != ""
}

// installReleaseOnSignal traps SIGINT/SIGTERM once and, before exiting,
// cancels the shared loop context (so the main loop and any in-flight
// session stop picking new work), then releases every task claimed by
// alias (treated as the agent-id) and clears the crash-recovery state
// file. A second signal triggers immediate exit.
//
// Cancel-first ordering matters: if release ran before cancel, the main
// loop could pick and start a new session during the release RTT and
// orphan its freshly-claimed task. Cancelling first stops the picker at
// its next ctx check, then we give the in-flight session a short window
// to release its own claim, then we sweep as a safety net.
//
// stateFile may be empty (single-session mode); logFn may be nil; cancel
// may be nil (no-op).
func installReleaseOnSignal(rc remoteConfig, alias, stateFile string, logFn func(string, ...any), cancel context.CancelFunc) {
	if logFn == nil {
		logFn = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "[%s] "+format+"\n",
				append([]any{time.Now().Format(time.RFC3339)}, args...)...)
		}
	}
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		logFn("received signal %s: cancelling loop and releasing claimed tasks...", sig)

		if cancel != nil {
			cancel()
		}

		// Give the in-flight session up to 2s to see the cancellation
		// and release its own claim. A second signal short-circuits.
		select {
		case <-time.After(2 * time.Second):
		case sig = <-sigCh:
			logFn("second signal %s: force exit", sig)
			os.Exit(130)
		}

		done := make(chan struct{})
		go func() {
			defer close(done)
			if alias != "" {
				released, err := releaseClaimedTasks(rc, alias)
				if len(released) > 0 {
					logFn("released %d task(s): %s", len(released), strings.Join(released, ","))
				}
				if err != nil {
					logFn("release error: %v", err)
				}
			}
			if stateFile != "" {
				os.Remove(stateFile)
			}
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logFn("release timeout (5s), exiting anyway")
		case sig = <-sigCh:
			logFn("second signal %s: force exit", sig)
		}
		os.Exit(130)
	}()
}

// releaseClaimedTasks lists every task claimed by agentID and asks the server
// to release them (admin release — empty agent_id in body). Best-effort: any
// error is logged and returned for callers to surface, but we keep going so a
// single bad request does not prevent the rest from being released.
func releaseClaimedTasks(rc remoteConfig, agentID string) (released []string, err error) {
	if !rc.valid() || agentID == "" {
		return nil, nil
	}
	client := &http.Client{Timeout: 5 * time.Second}

	listURL := fmt.Sprintf("%s/api/agents/%s/tasks", rc.url, url.PathEscape(agentID))
	req, _ := http.NewRequest("GET", listURL, nil)
	req.Header.Set("X-Api-Key", rc.key)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("list-agent-tasks: HTTP %d", resp.StatusCode)
	}

	var listBody struct {
		Tasks []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"tasks"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listBody); err != nil {
		return nil, err
	}

	for _, t := range listBody.Tasks {
		if t.Status == "done" {
			continue
		}
		relURL := fmt.Sprintf("%s/api/tasks/%s/release", rc.url, url.PathEscape(t.ID))
		body := bytes.NewBufferString(`{"agent_id":""}`)
		r, _ := http.NewRequest("POST", relURL, body)
		r.Header.Set("X-Api-Key", rc.key)
		r.Header.Set("Content-Type", "application/json")
		rr, rerr := client.Do(r)
		if rerr != nil {
			err = rerr
			continue
		}
		rr.Body.Close()
		if rr.StatusCode != 200 && rr.StatusCode != 204 {
			err = fmt.Errorf("release-task %s: HTTP %d", t.ID, rr.StatusCode)
			continue
		}
		released = append(released, t.ID)
	}
	return released, err
}

func claudeProjectDir() string {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	// claude CLI replaces / with - and keeps the leading dash (e.g. cwd
	// /home/foo → slug -home-foo). Stripping it sends the tailer to an
	// empty sibling directory and silently drops every event.
	slug := strings.ReplaceAll(cwd, "/", "-")
	return filepath.Join(home, ".claude", "projects", slug)
}

// runLoop implements the --loop behavior: pick work, run sessions, repeat.
func runLoop(rc remoteConfig, alias string, chrome bool) error {
	stateFile := ".zdx/cache/claude-work-state"
	logFile := ".zdx/logs/claude-work.log"
	os.MkdirAll(".zdx/logs", 0o755)
	os.MkdirAll(".zdx/cache", 0o755)

	logf, _ := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	defer logf.Close()

	log := func(format string, args ...any) {
		msg := fmt.Sprintf(format, args...)
		line := fmt.Sprintf("[%s] %s\n", time.Now().Format(time.RFC3339), msg)
		fmt.Print(line)
		if logf != nil {
			logf.WriteString(line)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	installReleaseOnSignal(rc, alias, stateFile, log, cancel)

	selfPath, _ := os.Executable()
	selfHash := fileHash(selfPath)

	for {
		if ctx.Err() != nil {
			return nil
		}

		// Self-update detection. Skip when either hash is empty: a transient
		// read error during a deploy (binary briefly unlinked/replaced) is
		// indistinguishable from a real change, and slicing an empty hash
		// panics. Retry on the next tick when both reads succeed.
		if h := fileHash(selfPath); h != "" && selfHash != "" && h != selfHash {
			log("self-update: %s → %s, re-execing", shortHash(selfHash), shortHash(h))
			if err := selfReexec(selfPath, os.Args); err != nil {
				log("re-exec failed: %v", err)
			}
		}

		var issueID, sid string
		resumed := false

		// Try to resume interrupted session
		if data, err := os.ReadFile(stateFile); err == nil {
			lines := strings.Split(strings.TrimSpace(string(data)), "\n")
			if len(lines) >= 2 && lines[0] != "" {
				savedIssue, savedSID := lines[0], lines[1]
				status := issueStatus(savedIssue)
				if status == "open" || status == "wip" {
					log("resuming interrupted session: issue=%s sid=%s", savedIssue, savedSID)
					issueID = savedIssue
					sid = savedSID
					resumed = true
				} else {
					log("stale state: %s is %s, clearing", savedIssue, status)
					os.Remove(stateFile)
				}
			}
		}

		if !resumed {
			todo, err := runDxTodoSolo("")
			if err != nil || todo == "" {
				log("idle; sleeping 60s")
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(60 * time.Second):
				}
				continue
			}
			log("todo solo:\n%s", todo)
			issueID = extractIssueID(todo)
			sid = uuid.New().String()
		}

		// Re-check cancellation before the point of no return (SESSION START
		// line + server-side session create). Without this, a signal that
		// arrives between solo-pick and session-start still leaves a stray
		// SESSION START in the log and on the server.
		if ctx.Err() != nil {
			return nil
		}

		// Save state for crash recovery
		os.WriteFile(stateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)

		prevSID := ""
		if resumed {
			prevSID = sid
			sid = uuid.New().String()
			os.WriteFile(stateFile, []byte(issueID+"\n"+sid+"\n"), 0o644)
			log("forking session: %s → %s", prevSID, sid)
		}

		log("──────────────────────────────────────────────")
		log("SESSION START  session=%s  issue=%s  resumed=%v", sid, issueID, resumed)
		log("──────────────────────────────────────────────")
		startTime := time.Now()

		if err := runSession(ctx, rc, sid, issueID, alias, chrome, prevSID, resumed); err != nil {
			log("session error: %v", err)
		}

		elapsed := time.Since(startTime)
		log("──────────────────────────────────────────────")
		log("SESSION END  session=%s  duration=%s", sid, elapsed.Truncate(time.Second))
		log("──────────────────────────────────────────────")

		os.Remove(stateFile)
	}
}

// runSession launches a single Claude CLI session and drives its lifecycle
// through the provider-agnostic RunLifecycle runner. Event tailing, WS
// streaming, and close are all owned by the shared runner — this wrapper
// only constructs a claudeAdapter and prints the post-session token summary.
func runSession(ctx context.Context, rc remoteConfig, sid, issueID, alias string, chrome bool, prevSID string, resumed bool) error {
	projDir := claudeProjectDir()
	_ = os.MkdirAll(projDir, 0o755)

	adapter := &claudeAdapter{
		projDir: projDir,
		chrome:  chrome,
		prevSID: prevSID,
		resumed: resumed,
		alias:   alias,
		exited:  make(chan struct{}),
	}

	_, err := RunLifecycle(ctx, adapter, rc, sid, issueID, alias, "claude-cli")

	// Print token usage summary from the on-disk transcripts regardless of
	// whether the lifecycle runner reached the server; useful in dev.
	printTokenSummary(
		filepath.Join(projDir, sid+".jsonl"),
		filepath.Join(projDir, sid, "subagents"),
	)
	return err
}

// ── Claude AgentAdapter ──────────────────────────────────────────────────

// claudeAdapter implements AgentAdapter against the real `claude` CLI. It
// launches the process with ZDX-aware environment vars and returns the
// transcript path that Claude writes its JSONL session to.
type claudeAdapter struct {
	projDir string
	chrome  bool
	prevSID string
	resumed bool
	alias   string

	proc       *exec.Cmd
	exited     chan struct{}
	exitedOnce sync.Once

	toolNamesMu sync.Mutex
	toolNames   map[string]string
}

func (a *claudeAdapter) Provider() string { return "claude" }

func (a *claudeAdapter) Start(ctx context.Context, sid, _, _ string) (string, error) {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return "", fmt.Errorf("claude CLI not found in PATH")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	if a.chrome {
		cmdArgs = append(cmdArgs, "--chrome")
	}
	if a.resumed && a.prevSID != "" {
		cmdArgs = append(cmdArgs, "--resume", a.prevSID, "--fork-session", "--session-id", sid, "-p", "/work")
	} else {
		cmdArgs = append(cmdArgs, "--session-id", sid, "-p", "/work")
	}

	a.proc = exec.Command(claudePath, cmdArgs...)
	a.proc.Stdin = os.Stdin
	a.proc.Stdout = os.Stdout
	a.proc.Stderr = os.Stderr
	a.proc.Env = append(os.Environ(),
		"ZDX_SESSION_ID="+sid,
		"ZDX_AGENT_ID="+a.alias,
	)

	if err := a.proc.Start(); err != nil {
		return "", err
	}

	// When the shared loop context cancels (e.g. SIGINT on the parent),
	// send SIGTERM to claude so the session wraps up cleanly; escalate to
	// SIGKILL after a grace window if it hasn't exited. Wait() observes the
	// termination and returns the real exit code, so SESSION END gets logged.
	go func() {
		<-ctx.Done()
		p := a.proc.Process
		if p == nil {
			return
		}
		_ = p.Signal(syscall.SIGTERM)
		select {
		case <-a.exited:
			return
		case <-time.After(10 * time.Second):
			_ = p.Kill()
		}
	}()

	return filepath.Join(a.projDir, sid+".jsonl"), nil
}

func (a *claudeAdapter) Wait() (int, error) {
	if a.proc == nil {
		return 1, fmt.Errorf("claudeAdapter.Wait: process not started")
	}
	err := a.proc.Wait()
	a.exitedOnce.Do(func() { close(a.exited) })
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode(), nil
		}
		return 1, err
	}
	return a.proc.ProcessState.ExitCode(), nil
}

func (a *claudeAdapter) SubagentDir(sid string) string {
	return filepath.Join(a.projDir, sid, "subagents")
}

func (a *claudeAdapter) ParseLine(line []byte, agentID string) (AgentEvent, error) {
	trimmed := bytes.TrimSpace(line)
	if len(trimmed) == 0 {
		return AgentEvent{}, fmt.Errorf("empty line")
	}
	var peek struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(trimmed, &peek); err != nil {
		return AgentEvent{}, err
	}
	return AgentEvent{
		EventType: peek.Type,
		EventJSON: json.RawMessage(append([]byte(nil), trimmed...)),
		AgentID:   agentID,
	}, nil
}

func (a *claudeAdapter) RenderEvent(eventJSON []byte) string {
	a.toolNamesMu.Lock()
	if a.toolNames == nil {
		a.toolNames = map[string]string{}
	}
	defer a.toolNamesMu.Unlock()
	return renderSessionEvent(eventJSON, a.toolNames)
}

// ANSI color codes. Emitted only when colorsEnabled is true (TTY + no NO_COLOR).
const (
	ansiReset   = "\033[0m"
	ansiDim     = "\033[2m"
	ansiBold    = "\033[1m"
	ansiCyan    = "\033[36m"
	ansiGreen   = "\033[32m"
	ansiRed     = "\033[31m"
	ansiMagenta = "\033[35m"
	ansiBlue    = "\033[34m"
)

var colorsEnabled = computeColorsEnabled()

func computeColorsEnabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func colorize(codes, s string) string {
	if !colorsEnabled || codes == "" {
		return s
	}
	return codes + s + ansiReset
}

// renderSessionEvent turns a single JSONL line from a Claude transcript into
// one or more human-readable lines (newline-separated) for stdout + agent.log.
// Returns "" for events that should not be rendered (queue-operation,
// attachment, non-tool-result user messages, malformed JSON).
//
// Output shape matches the retired bin/claude-work-render:
//
//	[Tool] detail            cyan bold (+ dim detail)
//	text                     plain
//	⟡ thinking               blue dim
//	● title                  magenta bold
//	✓ tool: content snippet  green dim (tool_result ok)
//	✗ tool: content snippet  red (tool_result err)
//
// toolNames accumulates tool_use id → name so later tool_result lines can
// label which tool returned.
func renderSessionEvent(line []byte, toolNames map[string]string) string {
	var evt struct {
		Type    string          `json:"type"`
		Title   string          `json:"title"`
		Message json.RawMessage `json:"message"`
	}
	if err := json.Unmarshal(line, &evt); err != nil {
		return ""
	}
	switch evt.Type {
	case "queue-operation", "attachment":
		return ""
	case "ai-title":
		t := strings.TrimSpace(evt.Title)
		if t == "" {
			return ""
		}
		return colorize(ansiBold+ansiMagenta, "● "+t)
	case "assistant":
		var m struct {
			Content []struct {
				Type     string          `json:"type"`
				Text     string          `json:"text"`
				Thinking string          `json:"thinking"`
				Name     string          `json:"name"`
				ID       string          `json:"id"`
				Input    json.RawMessage `json:"input"`
			} `json:"content"`
		}
		if err := json.Unmarshal(evt.Message, &m); err != nil {
			return ""
		}
		var lines []string
		for _, c := range m.Content {
			switch c.Type {
			case "tool_use":
				if c.ID != "" && c.Name != "" {
					toolNames[c.ID] = c.Name
				}
				lines = append(lines, renderToolUse(c.Name, c.Input))
			case "text":
				s := flattenTrunc(c.Text, 200)
				if s == "" {
					continue
				}
				lines = append(lines, s)
			case "thinking":
				s := flattenTrunc(c.Thinking, 100)
				if s == "" {
					continue
				}
				lines = append(lines, colorize(ansiDim+ansiBlue, "⟡ "+s))
			}
		}
		return strings.Join(lines, "\n")
	case "user":
		var m struct {
			Content json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(evt.Message, &m); err != nil {
			return ""
		}
		raw := bytes.TrimSpace(m.Content)
		if len(raw) == 0 || raw[0] != '[' {
			return ""
		}
		var arr []struct {
			Type      string          `json:"type"`
			ToolUseID string          `json:"tool_use_id"`
			IsError   bool            `json:"is_error"`
			Content   json.RawMessage `json:"content"`
		}
		if err := json.Unmarshal(raw, &arr); err != nil {
			return ""
		}
		var lines []string
		for _, c := range arr {
			if c.Type != "tool_result" {
				continue
			}
			name := toolNames[c.ToolUseID]
			if name == "" {
				name = "tool"
			}
			snippet := toolResultSnippet(c.Content, 120)
			if c.IsError {
				label := "✗ " + name
				if snippet != "" {
					label += ": " + snippet
				}
				lines = append(lines, colorize(ansiRed, label))
			} else {
				label := "✓ " + name
				if snippet != "" {
					label += ": " + snippet
				}
				lines = append(lines, colorize(ansiDim+ansiGreen, label))
			}
		}
		return strings.Join(lines, "\n")
	}
	return ""
}

// flattenTrunc collapses newlines to spaces, trims, and truncates to n runes
// with a trailing ellipsis. Returns "" if the input is blank.
func flattenTrunc(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) > n {
		return string(r[:n]) + "…"
	}
	return s
}

// toolResultSnippet extracts a short text snippet from tool_result content,
// which can be either a JSON string or an array of content blocks (text /
// image / etc). Returns "" when no usable text is present.
func toolResultSnippet(content json.RawMessage, n int) string {
	raw := bytes.TrimSpace(content)
	if len(raw) == 0 {
		return ""
	}
	switch raw[0] {
	case '"':
		var s string
		if json.Unmarshal(raw, &s) != nil {
			return ""
		}
		return flattenTrunc(s, n)
	case '[':
		var blocks []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if json.Unmarshal(raw, &blocks) != nil {
			return ""
		}
		var parts []string
		for _, b := range blocks {
			if b.Text != "" {
				parts = append(parts, b.Text)
			}
		}
		return flattenTrunc(strings.Join(parts, " "), n)
	}
	return ""
}

func renderToolUse(name string, input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	summary := ""
	switch name {
	case "Bash":
		if s, ok := in["command"].(string); ok {
			summary = flattenTrunc(s, 80)
		}
	case "Read", "Edit", "Write", "NotebookEdit":
		if s, ok := in["file_path"].(string); ok {
			summary = s
		}
	case "Grep", "Glob":
		if s, ok := in["pattern"].(string); ok {
			summary = s
		}
	case "Agent":
		if s, ok := in["description"].(string); ok && s != "" {
			summary = s
		} else if s, ok := in["subagent_type"].(string); ok {
			summary = s
		}
	case "WebFetch", "WebSearch":
		if s, ok := in["url"].(string); ok {
			summary = s
		} else if s, ok := in["query"].(string); ok {
			summary = s
		}
	}
	if name == "" {
		name = "tool"
	}
	label := colorize(ansiBold+ansiCyan, "["+name+"]")
	if summary == "" {
		return label
	}
	return label + " " + colorize(ansiDim, summary)
}

func parseSubagentMeta(jsonlPath string) (id, agentType, desc string) {
	base := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")
	id = base

	metaPath := strings.TrimSuffix(jsonlPath, ".jsonl") + ".meta.json"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		// Meta file might not exist yet, wait a bit
		for i := 0; i < 5; i++ {
			time.Sleep(500 * time.Millisecond)
			data, err = os.ReadFile(metaPath)
			if err == nil {
				break
			}
		}
	}
	if err == nil {
		var meta struct {
			AgentType   string `json:"agentType"`
			Description string `json:"description"`
		}
		if json.Unmarshal(data, &meta) == nil {
			agentType = meta.AgentType
			desc = meta.Description
		}
	}
	return
}

type tokenUsage struct {
	Input       int64
	Output      int64
	CacheRead   int64
	CacheCreate int64
}

func printTokenSummary(sfile, subagentDir string) {
	total := parseTokenUsage(sfile)

	matches, _ := filepath.Glob(filepath.Join(subagentDir, "agent-*.jsonl"))
	for _, m := range matches {
		sub := parseTokenUsage(m)
		total.Input += sub.Input
		total.Output += sub.Output
		total.CacheRead += sub.CacheRead
		total.CacheCreate += sub.CacheCreate
	}

	if total.Input+total.Output > 0 {
		fmt.Printf("tokens: input=%d output=%d cache_read=%d cache_create=%d\n",
			total.Input, total.Output, total.CacheRead, total.CacheCreate)
	}
}

func parseTokenUsage(path string) tokenUsage {
	var t tokenUsage
	f, err := os.Open(path)
	if err != nil {
		return t
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		var ev struct {
			Type    string `json:"type"`
			Message struct {
				Usage struct {
					InputTokens              int64 `json:"input_tokens"`
					OutputTokens             int64 `json:"output_tokens"`
					CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
					CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(scanner.Bytes(), &ev) == nil && ev.Type == "assistant" {
			t.Input += ev.Message.Usage.InputTokens
			t.Output += ev.Message.Usage.OutputTokens
			t.CacheRead += ev.Message.Usage.CacheReadInputTokens
			t.CacheCreate += ev.Message.Usage.CacheCreationInputTokens
		}
	}
	return t
}

func runDxTodoSolo(issue string) (string, error) {
	var args []string
	args = append(args, "todo", "solo")
	if issue != "" {
		args = append(args, "--issue="+issue)
	}
	dxPath, err := os.Executable()
	if err != nil {
		return "", err
	}
	out, err := exec.Command(dxPath, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func issueStatus(issueID string) string {
	dxPath, _ := os.Executable()
	out, err := exec.Command(dxPath, "todo", "show", issueID).CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Status:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[len(parts)-1]
			}
		}
	}
	return ""
}

func extractIssueID(text string) string {
	for _, word := range strings.Fields(text) {
		if strings.HasPrefix(word, "IS-") {
			return word
		}
	}
	return ""
}

func fileHash(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h)
}

func shortHash(s string) string {
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// selfReexec replaces the current process image with a fresh exec of the
// binary. syscall.Exec does not return on success, so any return is an error
// and the caller should keep the old binary running until the next tick.
func selfReexec(path string, args []string) error {
	return syscall.Exec(path, args, os.Environ())
}
