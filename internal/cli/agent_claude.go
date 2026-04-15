package cli

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
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
			sid := uuid.New().String()
			return runSession(rc, sid, issue, alias, chrome, "", false)
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

func claudeProjectDir() string {
	cwd, _ := os.Getwd()
	home, _ := os.UserHomeDir()
	slug := strings.ReplaceAll(cwd, "/", "-")
	if strings.HasPrefix(slug, "-") {
		slug = slug[1:]
	}
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

	selfPath, _ := os.Executable()
	selfHash := fileHash(selfPath)

	for {
		// Self-update detection
		if h := fileHash(selfPath); h != selfHash {
			log("self-update: %s → %s, re-execing", selfHash[:8], h[:8])
			if err := syscallExec(selfPath, os.Args); err != nil {
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
				time.Sleep(60 * time.Second)
				continue
			}
			log("todo solo:\n%s", todo)
			issueID = extractIssueID(todo)
			sid = uuid.New().String()
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

		if err := runSession(rc, sid, issueID, alias, chrome, prevSID, resumed); err != nil {
			log("session error: %v", err)
		}

		elapsed := time.Since(startTime)
		log("──────────────────────────────────────────────")
		log("SESSION END  session=%s  duration=%s", sid, elapsed.Truncate(time.Second))
		log("──────────────────────────────────────────────")

		os.Remove(stateFile)
	}
}

// runSession launches a single Claude CLI session with streaming.
func runSession(rc remoteConfig, sid, issueID, alias string, chrome bool, prevSID string, resumed bool) error {
	projDir := claudeProjectDir()
	os.MkdirAll(projDir, 0o755)
	sfile := filepath.Join(projDir, sid+".jsonl")

	// Start streaming to server
	var streamCancel func()
	if rc.valid() {
		streamCancel = startSessionStream(rc, sid, issueID, alias, sfile)
	}

	// Start subagent watcher
	subagentDir := filepath.Join(projDir, sid, "subagents")
	var subCancel func()
	if rc.valid() {
		subCancel = startSubagentWatcher(rc, sid, issueID, alias, subagentDir)
	}

	// Build claude command
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH")
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, "--dangerously-skip-permissions")
	if chrome {
		cmdArgs = append(cmdArgs, "--chrome")
	}
	if resumed && prevSID != "" {
		cmdArgs = append(cmdArgs, "--resume", prevSID, "--fork-session", "--session-id", sid, "-p", "/work")
	} else {
		cmdArgs = append(cmdArgs, "--session-id", sid, "-p", "/work")
	}

	proc := exec.Command(claudePath, cmdArgs...)
	proc.Stdin = os.Stdin
	proc.Stdout = os.Stdout
	proc.Stderr = os.Stderr

	err = proc.Run()

	// Stop streaming
	if streamCancel != nil {
		streamCancel()
	}
	if subCancel != nil {
		subCancel()
	}

	// Upload session (batch fallback for any missed events)
	if rc.valid() {
		uploadSession(rc, sid, sfile, issueID, alias)
	}

	// Print token usage summary
	printTokenSummary(sfile, filepath.Join(projDir, sid, "subagents"))

	return err
}

// startSessionStream tails the session JSONL file and streams it to the server.
func startSessionStream(rc remoteConfig, sid, issueID, alias, sfile string) func() {
	done := make(chan struct{})
	go func() {
		// Wait for session file to appear
		for i := 0; i < 120; i++ {
			if _, err := os.Stat(sfile); err == nil {
				break
			}
			select {
			case <-done:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}

		f, err := os.Open(sfile)
		if err != nil {
			return
		}
		defer f.Close()

		u := fmt.Sprintf("%s/api/dx/claude/sessions/ingest/stream?slug=%s&session_id=%s&issue_id=%s&alias=%s",
			rc.url, url.QueryEscape(rc.slug), url.QueryEscape(sid),
			url.QueryEscape(issueID), url.QueryEscape(alias))

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			reader := bufio.NewReader(f)
			for {
				select {
				case <-done:
					return
				default:
				}
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					pw.Write(line)
				}
				if err != nil {
					time.Sleep(200 * time.Millisecond)
					continue
				}
			}
		}()

		req, _ := http.NewRequest("POST", u, pr)
		req.Header.Set("X-Api-Key", rc.key)
		req.Header.Set("Content-Type", "application/x-ndjson")
		client := &http.Client{Timeout: 0}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()

	return func() { close(done) }
}

// startSubagentWatcher scans for new subagent JSONL files and streams them.
func startSubagentWatcher(rc remoteConfig, sid, issueID, alias, baseDir string) func() {
	done := make(chan struct{})
	var mu sync.Mutex
	known := make(map[string]bool)
	var cancels []func()

	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(2 * time.Second):
			}

			matches, _ := filepath.Glob(filepath.Join(baseDir, "agent-*.jsonl"))
			mu.Lock()
			for _, m := range matches {
				if known[m] {
					continue
				}
				known[m] = true

				agentID, agentType, agentDesc := parseSubagentMeta(m)
				u := fmt.Sprintf("%s/api/dx/claude/sessions/ingest/stream?slug=%s&session_id=%s&issue_id=%s&alias=%s&agent_id=%s&agent_type=%s&agent_description=%s",
					rc.url, url.QueryEscape(rc.slug), url.QueryEscape(sid),
					url.QueryEscape(issueID), url.QueryEscape(alias),
					url.QueryEscape(agentID), url.QueryEscape(agentType), url.QueryEscape(agentDesc))

				cancel := streamFile(m, u, rc.key, done)
				cancels = append(cancels, cancel)
			}
			mu.Unlock()
		}
	}()

	return func() {
		close(done)
		for _, c := range cancels {
			c()
		}
	}
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

func streamFile(path, u, apiKey string, done <-chan struct{}) func() {
	fileDone := make(chan struct{})
	go func() {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		pr, pw := io.Pipe()
		go func() {
			defer pw.Close()
			reader := bufio.NewReader(f)
			for {
				select {
				case <-done:
					return
				case <-fileDone:
					return
				default:
				}
				line, err := reader.ReadBytes('\n')
				if len(line) > 0 {
					pw.Write(line)
				}
				if err != nil {
					time.Sleep(200 * time.Millisecond)
					continue
				}
			}
		}()

		req, _ := http.NewRequest("POST", u, pr)
		req.Header.Set("X-Api-Key", apiKey)
		req.Header.Set("Content-Type", "application/x-ndjson")
		client := &http.Client{Timeout: 0}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}()
	return func() { close(fileDone) }
}

func uploadSession(rc remoteConfig, sid, sfile, issueID, alias string) {
	if _, err := os.Stat(sfile); err != nil {
		return
	}
	data, err := os.ReadFile(sfile)
	if err != nil {
		return
	}

	u := fmt.Sprintf("%s/api/dx/claude/sessions/ingest?slug=%s&session_id=%s&issue_id=%s&alias=%s",
		rc.url, url.QueryEscape(rc.slug), url.QueryEscape(sid),
		url.QueryEscape(issueID), url.QueryEscape(alias))

	req, _ := http.NewRequest("POST", u, strings.NewReader(string(data)))
	req.Header.Set("X-Api-Key", rc.key)
	req.Header.Set("Content-Type", "application/x-ndjson")
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err == nil {
		resp.Body.Close()
		fmt.Printf("upload: session %s → HTTP %d\n", sid, resp.StatusCode)
	}

	// Upload subagent transcripts
	projDir := claudeProjectDir()
	subDir := filepath.Join(projDir, sid, "subagents")
	uploadSubagents(rc, sid, issueID, alias, subDir)
}

func uploadSubagents(rc remoteConfig, sid, issueID, alias, subDir string) {
	matches, _ := filepath.Glob(filepath.Join(subDir, "agent-*.jsonl"))
	for _, m := range matches {
		agentID, agentType, agentDesc := parseSubagentMeta(m)
		data, err := os.ReadFile(m)
		if err != nil {
			continue
		}
		u := fmt.Sprintf("%s/api/dx/claude/sessions/ingest?slug=%s&session_id=%s&issue_id=%s&alias=%s&agent_id=%s&agent_type=%s&agent_description=%s",
			rc.url, url.QueryEscape(rc.slug), url.QueryEscape(sid),
			url.QueryEscape(issueID), url.QueryEscape(alias),
			url.QueryEscape(agentID), url.QueryEscape(agentType), url.QueryEscape(agentDesc))

		req, _ := http.NewRequest("POST", u, strings.NewReader(string(data)))
		req.Header.Set("X-Api-Key", rc.key)
		req.Header.Set("Content-Type", "application/x-ndjson")
		client := &http.Client{Timeout: 30 * time.Second}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
			fmt.Printf("upload: subagent %s → HTTP %d\n", agentID, resp.StatusCode)
		}

		// Recurse into nested subagents
		nestedDir := strings.TrimSuffix(m, ".jsonl") + "/subagents"
		uploadSubagents(rc, sid, issueID, alias, nestedDir)
	}
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

func syscallExec(path string, args []string) error {
	cmd := exec.Command(path, args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
