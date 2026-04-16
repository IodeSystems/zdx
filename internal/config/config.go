// Package config loads .zdx/config.yaml and ~/.zdx/credentials.
package config

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config mirrors the .zdx/config.yaml schema.
type Config struct {
	Remote     Remote               `yaml:"remote"`
	Hooks      Hooks                `yaml:"hooks"`
	Components map[string]Component `yaml:"components"`
	LLMLocal   LLMLocal             `yaml:"llm_local"`
}

// LLMLocal configures the local-LLM provider used by `dx agent local`.
type LLMLocal struct {
	BaseURL        string `yaml:"base_url"`
	Model          string `yaml:"model"`
	APIKey         string `yaml:"api_key"`
	TimeoutSeconds int    `yaml:"timeout_seconds"`
}

const (
	defaultLLMLocalBaseURL = "http://192.168.1.76:8111"
	defaultLLMLocalModel   = "qwen3-30b-a3b"
	defaultLLMLocalTimeout = 120
)

// ResolvedLLMLocal returns the local-LLM config with defaults and env overrides applied.
// Env: DX_LLM_LOCAL_BASE_URL, DX_LLM_LOCAL_MODEL, DX_LLM_LOCAL_API_KEY.
func (c *Config) ResolvedLLMLocal() LLMLocal {
	var l LLMLocal
	if c != nil {
		l = c.LLMLocal
	}
	if v := os.Getenv("DX_LLM_LOCAL_BASE_URL"); v != "" {
		l.BaseURL = v
	}
	if v := os.Getenv("DX_LLM_LOCAL_MODEL"); v != "" {
		l.Model = v
	}
	if v := os.Getenv("DX_LLM_LOCAL_API_KEY"); v != "" {
		l.APIKey = v
	}
	if l.BaseURL == "" {
		l.BaseURL = defaultLLMLocalBaseURL
	}
	if l.Model == "" {
		l.Model = defaultLLMLocalModel
	}
	if l.TimeoutSeconds <= 0 {
		l.TimeoutSeconds = defaultLLMLocalTimeout
	}
	return l
}

type Remote struct {
	URL  string `yaml:"url"`
	Slug string `yaml:"slug"`
}

type Hooks struct {
	PreCommit []string `yaml:"pre-commit"`
}

type Component struct {
	Build Build            `yaml:"build"`
	Test  map[string]Suite `yaml:"test"`
	Lint  Lint             `yaml:"lint"`
	Watch map[string]Watch `yaml:"watch"`
	Close Close            `yaml:"close"`
}

type Close struct {
	Steps []Step `yaml:"steps"`
}

type Build struct {
	Steps []Step `yaml:"steps"`
}

type Step struct {
	Name string `yaml:"name"`
	Run  string `yaml:"run"`
	CWD  string `yaml:"cwd"`
}

type Suite struct {
	Runner   string `yaml:"runner"`
	Run      string `yaml:"run"`
	Setup    string `yaml:"setup"`
	Teardown string `yaml:"teardown"`
	CWD      string `yaml:"cwd"`
}

type Lint struct {
	Zig      LintZig `yaml:"zig"`
	External []Step  `yaml:"external"`
}

type LintZig struct {
	Dirs []string `yaml:"dirs"`
}

type Watch struct {
	Dirs    []string `yaml:"dirs"`
	Include string   `yaml:"include"`
	Run     string   `yaml:"run"`
}

// Load reads .zdx/config.yaml from cwd. Returns nil if not found.
func Load() *Config {
	data, err := os.ReadFile(".zdx/config.yaml")
	if err != nil {
		return nil
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// RemoteURL returns the configured remote URL (env overrides config).
func (c *Config) RemoteURL() string {
	if v := os.Getenv("DX_REMOTE_URL"); v != "" {
		return v
	}
	if c != nil {
		return c.Remote.URL
	}
	return ""
}

// RemoteSlug returns the configured project slug (env overrides config).
func (c *Config) RemoteSlug() string {
	if v := os.Getenv("DX_REMOTE_SLUG"); v != "" {
		return v
	}
	if c != nil && c.Remote.Slug != "" {
		return c.Remote.Slug
	}
	return contextSlug()
}

// RemoteAPIKey returns the API key from env, then .zdx/credentials.
func RemoteAPIKey() string {
	if v := os.Getenv("DX_REMOTE_API_KEY"); v != "" {
		return v
	}
	return ReadCredentials()
}

// ReadCredentials reads the API key from .zdx/credentials.
func ReadCredentials() string {
	b, err := os.ReadFile(".zdx/credentials")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// DaemonConn holds a resolved local daemon connection.
type DaemonConn struct {
	URL   string
	Token string
}

// ReadDaemonConn checks ~/.zdx/daemon.{port,token}.
func ReadDaemonConn() *DaemonConn {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	portBytes, err := os.ReadFile(filepath.Join(home, ".zdx", "daemon.port"))
	if err != nil {
		return nil
	}
	port := strings.TrimSpace(string(portBytes))
	if port == "" {
		return nil
	}
	tokenBytes, _ := os.ReadFile(filepath.Join(home, ".zdx", "daemon.token"))
	return &DaemonConn{
		URL:   "http://localhost:" + port,
		Token: strings.TrimSpace(string(tokenBytes)),
	}
}

// contextSlug reads slug from .zdx/context (fallback).
func contextSlug() string {
	b, err := os.ReadFile(".zdx/context")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "slug="); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// ActiveComponent returns the resolved component from flag > DX_COMPONENT env > .zdx/context.
func ActiveComponent(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("DX_COMPONENT"); v != "" {
		return v
	}
	b, err := os.ReadFile(".zdx/context")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if rest, ok := strings.CutPrefix(line, "component="); ok {
			return strings.TrimSpace(rest)
		}
	}
	return ""
}

// WriteContext writes component (and optionally slug) to .zdx/context.
func WriteContext(component, slug string) error {
	var lines []string
	if component != "" {
		lines = append(lines, "component="+component)
	}
	if slug != "" {
		lines = append(lines, "slug="+slug)
	}
	return os.WriteFile(".zdx/context", []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

// AllBuildSteps aggregates build steps across components (optionally filtered).
func (c *Config) AllBuildSteps(component string) []NamedStep {
	var out []NamedStep
	for name, comp := range c.Components {
		if component != "" && name != component {
			continue
		}
		for _, s := range comp.Build.Steps {
			out = append(out, NamedStep{Component: name, Step: s})
		}
	}
	return out
}

// AllCloseSteps aggregates close steps across components (optionally filtered).
func (c *Config) AllCloseSteps(component string) []NamedStep {
	var out []NamedStep
	for name, comp := range c.Components {
		if component != "" && name != component {
			continue
		}
		for _, s := range comp.Close.Steps {
			out = append(out, NamedStep{Component: name, Step: s})
		}
	}
	return out
}

// AllTestSuites aggregates test suites across components (optionally filtered).
func (c *Config) AllTestSuites(component string) []NamedSuite {
	var out []NamedSuite
	for name, comp := range c.Components {
		if component != "" && name != component {
			continue
		}
		for suiteName, suite := range comp.Test {
			out = append(out, NamedSuite{Component: name, Name: suiteName, Suite: suite})
		}
	}
	return out
}

// AllWatches aggregates watch entries across components (optionally filtered).
func (c *Config) AllWatches(component string) []NamedWatch {
	var out []NamedWatch
	for compName, comp := range c.Components {
		if component != "" && compName != component {
			continue
		}
		for watchName, w := range comp.Watch {
			out = append(out, NamedWatch{Component: compName, Name: watchName, Watch: w})
		}
	}
	return out
}

// NamedStep is a build step with its component label.
type NamedStep struct {
	Component string
	Step
}

// NamedSuite is a test suite with its component and suite name.
type NamedSuite struct {
	Component string
	Name      string
	Suite
}

// NamedWatch is a watch entry with its component and watch name.
type NamedWatch struct {
	Component string
	Name      string
	Watch
}
