package agent

import (
	"context"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/iodesystems/zdx-go/internal/config"
)

// ResumeRunnerConfig is the externally-visible inputs needed to resume a session.
// cmd/dx-agent passes these from the daemon's ControlMsg so the task loop can
// call Take without leaking unexported types (remoteConfig, modelSelector).
type ResumeRunnerConfig struct {
	ServerURL string
	APIKey    string
	AgentID   string
	Alias     string
	WorkDir   string // worktree path; .zdx/config.yaml is read from here for slug + agent config
	SessionID string // becomes TakeConfig.ResumeSID
	IssueID   string // becomes TakeConfig.ResumeIssueID
	LogFn     func(string, ...any)
}

// RunResume loads project config from WorkDir/.zdx/config.yaml, builds the
// internal remoteConfig/modelSelector, and calls Take with ResumeIssueID and
// ResumeSID set. It returns a TakeResult so the caller can log success/failure.
func RunResume(ctx context.Context, cfg ResumeRunnerConfig) TakeResult {
	rc, agentCfg := loadResumeConfig(cfg)
	sel := modelSelector{agentCfg: agentCfg}

	stateFile := ".zdx/cache/claude-work-state"
	if cfg.WorkDir != "" {
		stateFile = filepath.Join(cfg.WorkDir, ".zdx", "cache", "claude-work-state")
	}

	return Take(ctx, TakeConfig{
		RC:            rc,
		AgentID:       cfg.AgentID,
		Alias:         cfg.Alias,
		AgentCfg:      agentCfg,
		ModelSel:      sel,
		StateFile:     stateFile,
		ResumeIssueID: cfg.IssueID,
		ResumeSID:     cfg.SessionID,
		LogFn:         cfg.LogFn,
	})
}

// loadResumeConfig reads .zdx/config.yaml from cfg.WorkDir to get the project
// slug and agent config. Falls back to cfg.ServerURL/APIKey for connectivity.
func loadResumeConfig(cfg ResumeRunnerConfig) (remoteConfig, config.AgentConfig) {
	rc := remoteConfig{
		url: cfg.ServerURL,
		key: cfg.APIKey,
	}

	var projectCfg *config.Config
	if cfg.WorkDir != "" {
		data, err := os.ReadFile(filepath.Join(cfg.WorkDir, ".zdx", "config.yaml"))
		if err == nil {
			var c config.Config
			if yaml.Unmarshal(data, &c) == nil {
				projectCfg = &c
			}
		}
	}

	if projectCfg != nil {
		rc.slug = projectCfg.RemoteSlug()
		return rc, projectCfg.ResolvedAgent()
	}
	return rc, (&config.Config{}).ResolvedAgent()
}
