// Package config loads .zdx/config.yaml and ~/.zdx/credentials.
package config

import (
	"errors"
	"fmt"
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
	Agent      AgentConfig          `yaml:"agent"`
	Deploy     DeployConfig         `yaml:"deploy"`
}

// Deploy-strategy constants. See IS-927 for the design.
const (
	DeployStrategyTrunk         = "trunk"
	DeployStrategyGateBranch    = "gate-branch"
	DeployStrategyReleaseBranch = "release-branch"
	DeployStrategyManual        = "manual"
)

// DeployConfig declares how a project's deploy pipeline is structured.
// Doctor's has_deploy_config rung consults Strategy to decide whether the
// project's branching model is satisfied without flipping defaults for
// projects that never set it.
type DeployConfig struct {
	Strategy string `yaml:"strategy"`
}

// ResolvedDeploy returns deploy config with the strategy normalized
// (lowercase + trimmed). Unknown non-empty values pass through so doctor
// can flag them as misconfigured. Empty preserves "unset" semantics.
func (c *Config) ResolvedDeploy() DeployConfig {
	var d DeployConfig
	if c != nil {
		d = c.Deploy
	}
	d.Strategy = strings.ToLower(strings.TrimSpace(d.Strategy))
	return d
}

// ValidDeployStrategy reports whether s is one of the recognized
// deploy.strategy values.
func ValidDeployStrategy(s string) bool {
	switch s {
	case DeployStrategyTrunk, DeployStrategyGateBranch, DeployStrategyReleaseBranch, DeployStrategyManual:
		return true
	}
	return false
}

// AgentConfig declares how agents run in this project.
type AgentConfig struct {
	ComposeFile     string `yaml:"compose_file"`     // docker-compose file for dev services (default: docker-compose.agent.yaml)
	DevDockerfile   string `yaml:"dev_dockerfile"`   // Dockerfile for the dev image (default: Dockerfile.agent)
	MaxWorktrees    int    `yaml:"max_worktrees"`    // concurrent agent slots (default: 4)
	LLMProvider     string `yaml:"llm_provider"`     // claude | local | server (default: claude)
	ClaudeModel     string `yaml:"claude_model"`     // model when provider=claude (default: claude-sonnet-4-6)
	LeaseMinutes    int    `yaml:"lease_minutes"`    // todo lease duration in minutes (default: 30)
	ContainerMemory string `yaml:"container_memory"` // docker --memory limit per agent slot (default: 4g)
	ContainerCPUs   string `yaml:"container_cpus"`   // docker --cpus limit per agent slot (default: 2)
}

// ResolvedAgent returns agent config with defaults applied.
func (c *Config) ResolvedAgent() AgentConfig {
	var a AgentConfig
	if c != nil {
		a = c.Agent
	}
	if a.ComposeFile == "" {
		a.ComposeFile = "docker-compose.agent.yaml"
	}
	if a.DevDockerfile == "" {
		a.DevDockerfile = "Dockerfile.agent"
	}
	if a.MaxWorktrees <= 0 {
		a.MaxWorktrees = 4
	}
	if a.LLMProvider == "" {
		a.LLMProvider = "claude"
	}
	if a.ClaudeModel == "" {
		a.ClaudeModel = "claude-sonnet-4-6"
	}
	if a.LeaseMinutes <= 0 {
		a.LeaseMinutes = 30
	}
	if a.ContainerMemory == "" {
		a.ContainerMemory = "4g"
	}
	if a.ContainerCPUs == "" {
		a.ContainerCPUs = "2"
	}
	return a
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
	Ship  Ship             `yaml:"ship"`
}

// Ship-strategy constants. See IS-793 for the design.
const (
	ShipStrategySimple      = "simple"
	ShipStrategyRollingPair = "rolling-pair"
	ShipStrategyMaintenance = "maintenance"
	ShipStrategyBlueGreen   = "blue-green"
)

// Ship declares how a component deploys. Stages run in declared order;
// a non-empty Strategy or Stages list means Ship is configured.
type Ship struct {
	Strategy    string            `yaml:"strategy"`
	SecretsFile string            `yaml:"secrets_file"` // IS-892: resolved at run time by secrets loader
	Env         map[string]string `yaml:"env"`
	Stages      []Stage           `yaml:"stages"`
}

// IsZero reports whether Ship has no user-supplied fields. Used to skip
// validation for components that don't declare a ship section.
func (s Ship) IsZero() bool {
	return s.Strategy == "" && s.SecretsFile == "" && len(s.Env) == 0 && len(s.Stages) == 0
}

// Stage is one step in a ship pipeline. Target, when non-empty, means
// the Run command executes on a remote host (ssh wrapper); Optional
// stages don't halt the pipeline on failure. Tags are free-form labels
// strategies may inspect — e.g. blue-green re-runs only stages tagged
// "verify" after the slot swap. The "build" tag marks stages that produce
// the deploy artifact (cross-compile, UI bundle, tarball); stages without
// "build" are deploy/verify stages. This enables --package (include only
// build-tagged stages) and --no-package (exclude build-tagged stages) in
// dx ship run. Finalize stages run after all main stages succeed; their
// failure is recorded but never propagates as an error.
// Exactly one of Run or Builtin must be set. Builtin names a harness-
// provided primitive (e.g. "deploy-event"); Run is a shell command.
type Stage struct {
	Name      string   `yaml:"name"`
	Run       string   `yaml:"run"`
	Builtin   string   `yaml:"builtin"` // harness primitive (IS-888/891/797); alternative to Run
	Target    string   `yaml:"target"`
	OnFailure string   `yaml:"on_failure"` // "abort" (default) | "continue" — IS-895 will wire this
	Optional  bool     `yaml:"optional"`
	Finalize  bool     `yaml:"finalize"` // post-success; failure never aborts the pipeline (IS-891)
	Tags      []string `yaml:"tags,omitempty"`
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

// Load reads .zdx/config.yaml from cwd. Returns nil if not found,
// unparseable, or fails validation. Validation errors are written to
// stderr so users see actionable feedback without callers needing to
// thread errors through every callsite. Use LoadAndValidate when the
// caller wants to consume errors directly (e.g. the ship harness).
func Load() *Config {
	cfg, err := LoadAndValidate()
	if err != nil {
		// Distinguish "no config" (silent) from "broken config" (loud).
		if !errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
		}
		return nil
	}
	return cfg
}

// LoadAndValidate reads .zdx/config.yaml, unmarshals it, and runs
// validate(). Returns os.ErrNotExist (wrapped) when the file is absent.
func LoadAndValidate() (*Config, error) {
	data, err := os.ReadFile(".zdx/config.yaml")
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse .zdx/config.yaml: %w", err)
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// validate runs structural checks across the parsed config. Today it
// only enforces the Ship schema (IS-794); add more concerns here as
// other component sections grow validation needs.
func (c *Config) validate() error {
	// Sort component names so error ordering is deterministic — tests
	// and humans both benefit. Iterating a map directly would yield
	// errors in random order on each run.
	names := make([]string, 0, len(c.Components))
	for name := range c.Components {
		names = append(names, name)
	}
	sortStrings(names)

	var errs []string
	for _, name := range names {
		comp := c.Components[name]
		if comp.Ship.IsZero() {
			continue
		}
		errs = append(errs, validateShip(name, comp.Ship)...)
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.New(strings.Join(errs, "\n"))
}

// sortStrings is a tiny in-place sort to avoid pulling sort just for
// determinism in validate().
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

// validateShip returns a flat list of error strings (one per problem)
// scoped to a single component. Errors include the components.<name>
// prefix so the Config-level aggregator doesn't need to re-prefix.
func validateShip(comp string, s Ship) []string {
	var errs []string
	prefix := fmt.Sprintf("components.%s.ship", comp)

	if s.Strategy != "" && !validShipStrategy(s.Strategy) {
		errs = append(errs, fmt.Sprintf("%s.strategy: unknown strategy %q (want one of: %s, %s, %s, %s)",
			prefix, s.Strategy,
			ShipStrategySimple, ShipStrategyRollingPair, ShipStrategyMaintenance, ShipStrategyBlueGreen))
	}

	if len(s.Stages) == 0 {
		errs = append(errs, fmt.Sprintf("%s.stages: must declare at least one stage", prefix))
		return errs
	}

	seen := make(map[string]int, len(s.Stages))
	for i, st := range s.Stages {
		stagePrefix := fmt.Sprintf("%s.stages[%d]", prefix, i)
		if st.Run == "" && st.Builtin == "" {
			errs = append(errs, fmt.Sprintf("%s: missing run or builtin", stagePrefix))
		}
		if st.Name == "" {
			// Unnamed stages can't be deduped meaningfully; report and
			// move on rather than collapsing them in `seen`.
			continue
		}
		if prev, dup := seen[st.Name]; dup {
			errs = append(errs, fmt.Sprintf("%s: duplicate stage name %q (also at stages[%d])",
				stagePrefix, st.Name, prev))
			continue
		}
		seen[st.Name] = i
	}
	return errs
}

func validShipStrategy(s string) bool {
	switch s {
	case ShipStrategySimple, ShipStrategyRollingPair, ShipStrategyMaintenance, ShipStrategyBlueGreen:
		return true
	}
	return false
}

// GlobalConfig mirrors the ~/.zdx/config.yaml schema for srcless (multi-project) agent operation.
type GlobalConfig struct {
	Remote GlobalRemote      `yaml:"remote"`
	Agent  GlobalAgentConfig `yaml:"agent"`
}

// GlobalRemote holds the remote URL and API key for srcless agents.
type GlobalRemote struct {
	URL    string `yaml:"url"`
	APIKey string `yaml:"api_key"`
}

// GlobalAgentConfig holds agent settings for srcless operation.
type GlobalAgentConfig struct {
	WorkDir      string `yaml:"work_dir"`
	ClaudeModel  string `yaml:"claude_model"`
	MaxWorktrees int    `yaml:"max_worktrees"`
	LeaseMinutes int    `yaml:"lease_minutes"`
}

// IsSrcless returns true when the agent is running outside any project
// checkout: no .zdx/config.yaml in cwd but ~/.zdx/config.yaml exists.
// Srcless agents resolve per-todo project slugs at claim time and clone
// each project on demand (see internal/cli/agent/srcless.go).
func IsSrcless() bool {
	return Load() == nil && LoadGlobal() != nil
}

// IsGlobalMode reports whether the DX_GLOBAL env var is truthy ("1" or "true").
// When set, downstream dx invocations prefer ~/.zdx/config.yaml + credentials
// over project config, matching `--global` on `dx agent`. Agent loops set
// DX_GLOBAL=1 on spawned Claude subprocesses so /work's inner dx calls inherit
// srcless mode without needing --global on every invocation.
func IsGlobalMode() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("DX_GLOBAL"))) {
	case "1", "true":
		return true
	}
	return false
}

// LoadGlobal reads ~/.zdx/config.yaml. Returns nil if not found or unreadable.
func LoadGlobal() *GlobalConfig {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(filepath.Join(home, ".zdx", "config.yaml"))
	if err != nil {
		return nil
	}
	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return &cfg
}

// WriteGlobal writes cfg to ~/.zdx/config.yaml, creating the directory if needed.
func WriteGlobal(cfg *GlobalConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".zdx")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o600)
}

// GlobalRemoteAPIKey returns the API key from env, then ~/.zdx/credentials.
func GlobalRemoteAPIKey() string {
	if v := os.Getenv("DX_REMOTE_API_KEY"); v != "" {
		return v
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(home, ".zdx", "credentials"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// WriteGlobalCredentials writes the API key to ~/.zdx/credentials.
func WriteGlobalCredentials(apiKey string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".zdx")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "credentials"), []byte(apiKey+"\n"), 0o600)
}

// ResolvedGlobalAgent returns GlobalAgentConfig with defaults applied.
func (c *GlobalConfig) ResolvedGlobalAgent() GlobalAgentConfig {
	a := c.Agent
	if a.WorkDir == "" {
		home, _ := os.UserHomeDir()
		a.WorkDir = filepath.Join(home, ".zdx", "projects")
	}
	if a.ClaudeModel == "" {
		a.ClaudeModel = "claude-sonnet-4-6"
	}
	if a.MaxWorktrees <= 0 {
		a.MaxWorktrees = 4
	}
	if a.LeaseMinutes <= 0 {
		a.LeaseMinutes = 30
	}
	return a
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
