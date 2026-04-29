package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// Finding is the result of running one check.
type Finding struct {
	Check    Check
	Rung     string
	Status   FindingStatus
	Message  string       // human-readable detail
	FixFunc  func() error // non-nil for auto-fixable findings
	Proposal string       // non-empty for proposable findings
}

type FindingStatus int

const (
	StatusPass FindingStatus = iota
	StatusFail
	StatusDeferred
)

func (s FindingStatus) String() string {
	switch s {
	case StatusPass:
		return "pass"
	case StatusFail:
		return "fail"
	case StatusDeferred:
		return "deferred"
	default:
		return "unknown"
	}
}

// ProjectState holds everything doctor needs to evaluate checks.
// Populated by querying the server/daemon via the CLI client.
type ProjectState struct {
	// Local filesystem
	ZdxDirExists     bool
	ConfigValid      bool
	CredentialsExist bool
	Config           *config.Config

	// Classification
	Classification Classification

	// Vision
	HasVision bool

	// Remote project state (from server)
	RemoteReachable    bool
	GoalCount          int
	ConstraintCount    int
	FeatureCount       int
	FeaturesWithSpecs  int
	FeaturesAttributed int
	UntriagedIssues    int
	SpecsCovered       int
	SpecsTotal         int
	GoalsQuantified    int
	GoalsTotal         int
	OverspeccedCount   int

	// Environment detection
	ClaudeInstalled bool
	DockerAvailable bool
	AgentConfigSet  bool

	// Agent-session health (from server)
	StaleAgentSessions        int   // sessions still open beyond the stale threshold
	StaleAgentSessionsMinutes int32 // threshold used for the check

	// Files
	HasReadme        bool
	HasLicense       bool
	HasChangelog     bool
	HasBuildSteps    bool
	HasDevDockerfile bool
	HasDevCompose    bool

	// Feature quality
	FeaturesImplDetail int // features whose name/desc looks like a code module

	// Code quality
	RawAPICallsGo    int // raw URL callsites in Go CLI
	RawAPICallsUI    int // raw fetch/post callsites in UI
	RawAPICallsFiles int // total files with raw callsites

	// Layered BDD test architecture
	SpecCount            int  // total spec count across features
	DemoTestResultsExist bool // at least one demo-layer test result recorded
	UsesStepDriver       bool // codebase has StepDriver usage in test files

	// Deploy pipeline
	ShipsFromDevDirectly bool // true when project deploys from dev/main without a gate/staging branch

	// Maturity questionnaire (from server)
	MaturityQuestions []dxclient.MaturityQuestion
	MaturityItems     []dxclient.MaturityItem

	// Deferred checks (from server)
	Deferred map[string]bool
}

// DetectLocal populates filesystem and environment checks.
func DetectLocal(state *ProjectState) {
	// .zdx/ exists
	if info, err := os.Stat(".zdx"); err == nil && info.IsDir() {
		state.ZdxDirExists = true
	}

	// Config valid
	cfg := config.Load()
	if cfg != nil {
		state.ConfigValid = true
		state.Config = cfg
		state.AgentConfigSet = cfg.Agent.LLMProvider != "" || cfg.Agent.ComposeFile != ""
		state.HasBuildSteps = len(cfg.AllBuildSteps("")) > 0
	}

	// Credentials
	if cred := config.ReadCredentials(); cred != "" {
		state.CredentialsExist = true
	}

	// Claude CLI
	if _, err := exec.LookPath("claude"); err == nil {
		state.ClaudeInstalled = true
	}

	// Docker
	if out, err := exec.Command("docker", "info").CombinedOutput(); err == nil && len(out) > 0 {
		state.DockerAvailable = true
	}

	// Files
	state.HasReadme = fileExists("README.md")
	state.HasLicense = fileExists("LICENSE") || fileExists("LICENSE.md") || fileExists("LICENSE.txt")
	state.HasChangelog = fileExists("CHANGELOG.md") || fileExists("CHANGELOG")
	state.HasDevDockerfile = fileExists("dev.Dockerfile") || fileExists("Dockerfile.dev")
	state.HasDevCompose = fileExists("docker-compose.dev.yml") || fileExists("docker-compose.dev.yaml")

	// Raw API callsites
	state.RawAPICallsGo = countRawCallsites("internal/cli/", `c\.Get("/api/\|c\.Post("/api/\|c\.Delete("/api/`, "*.go")
	state.RawAPICallsUI = countRawCallsites("ui/src/", `apiFetch\|apiPost`, "*.ts") +
		countRawCallsites("ui/src/", `apiFetch\|apiPost`, "*.tsx")

	// StepDriver adoption
	state.UsesStepDriver = codebaseUsesStepDriver()

	// Gate-branch detection: ships from dev directly if a deploy script exists
	// but no staging/gate/release branch is present in the repo.
	state.ShipsFromDevDirectly = detectShipsFromDevDirectly()
}

func codebaseUsesStepDriver() bool {
	cmd := exec.Command("grep", "-r", "--include=*_test.go", "-l", "StepDriver", ".")
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func detectShipsFromDevDirectly() bool {
	if !fileExists("bin/ship") {
		return false
	}
	out, err := exec.Command("git", "branch", "-a").Output()
	if err != nil {
		return true // has deploy script, can't check branches → assume direct
	}
	for _, line := range strings.Split(string(out), "\n") {
		b := strings.TrimSpace(line)
		b = strings.TrimPrefix(b, "* ")
		b = strings.TrimPrefix(b, "remotes/origin/")
		switch b {
		case "staging", "gate", "release", "pre-production", "pre-prod":
			return false
		}
	}
	return true
}

// implDetailKeywords are technical implementation words that signal a feature
// is describing code structure rather than a user-visible capability.
var implDetailKeywords = []string{
	"layer", "middleware", "handler", "module", "refactor", "migration",
	"infrastructure", "impl", "implementation", "schema", "endpoint",
	"service-layer", "cache-layer", "api-gateway", "backend", "worker",
	"pipeline", "scheduler", "cleanup", "cron", "webhook",
}

// IsImplDetailFeature returns true when the feature name or description
// contains implementation-detail keywords, indicating the feature describes
// code structure rather than a user-visible outcome.
func IsImplDetailFeature(name, desc string) bool {
	lower := strings.ToLower(name + " " + desc)
	for _, kw := range implDetailKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func countRawCallsites(dir, pattern, glob string) int {
	cmd := exec.Command("grep", "-rn", pattern, dir, "--include="+glob)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) != "" &&
			!strings.Contains(line, "api.gen.ts") &&
			!strings.Contains(line, "node_modules") {
			count++
		}
	}
	return count
}

// Evaluate runs all checks for the project's classification and returns findings.
func Evaluate(state *ProjectState) []Finding {
	class := state.Classification
	if class == "" {
		class = ClassTool // default if unset
	}
	vine := Vine(class)

	var findings []Finding
	for _, rung := range vine {
		for _, check := range rung.Checks {
			f := evaluateCheck(check, rung.Name, state)
			findings = append(findings, f)
		}
	}
	return findings
}

func evaluateCheck(check Check, rung string, state *ProjectState) Finding {
	f := Finding{Check: check, Rung: rung}

	if state.Deferred[check.Name] {
		f.Status = StatusDeferred
		f.Message = "deferred by user"
		return f
	}

	pass, msg, fixFunc, proposal := runCheck(check.Name, state)
	if pass {
		f.Status = StatusPass
		f.Message = msg
	} else {
		f.Status = StatusFail
		f.Message = msg
		f.FixFunc = fixFunc
		f.Proposal = proposal
	}
	return f
}

func runCheck(name string, state *ProjectState) (pass bool, msg string, fixFunc func() error, proposal string) {
	switch name {
	// ── scaffold ──
	case "zdx_dir_exists":
		if state.ZdxDirExists {
			return true, "", nil, ""
		}
		return false, ".zdx/ directory missing", func() error {
			return os.MkdirAll(".zdx", 0o755)
		}, ""

	case "config_valid":
		if state.ConfigValid {
			return true, "", nil, ""
		}
		return false, ".zdx/config.yaml missing or invalid", func() error {
			return os.WriteFile(".zdx/config.yaml", []byte("remote:\n  url: \"\"\n  slug: \"\"\n"), 0o644)
		}, ""

	case "credentials_exist":
		if state.CredentialsExist {
			return true, "", nil, ""
		}
		return false, "no API key in .zdx/credentials", nil,
			"Run `dx login` or `dx integrate --url <server> --api-key <key>` to set credentials"

	case "remote_reachable":
		if state.RemoteReachable {
			return true, "", nil, ""
		}
		return false, "remote server not reachable", nil, ""

	// ── identity ──
	case "classification_set":
		if state.Classification != "" {
			return true, string(state.Classification), nil, ""
		}
		return false, "project classification not set", nil,
			"Run `dx doctor` to choose a classification (library/tool/service/saas/site)"

	case "has_vision":
		if state.HasVision {
			return true, "vision set", nil, ""
		}
		return false, "no project vision defined", nil,
			"Run `dx vision set --title '<tagline>' --desc '<who, what, why>'` — a guiding star that frames all goals and features"

	case "has_goals":
		if state.GoalCount > 0 {
			return true, fmt.Sprintf("%d goals", state.GoalCount), nil, ""
		}
		return false, "no goals defined", nil,
			"Run `dx goal add <title>` to define what this project is trying to achieve"

	case "has_constraints":
		if state.ConstraintCount > 0 {
			return true, fmt.Sprintf("%d constraints", state.ConstraintCount), nil, ""
		}
		return false, "no constraints defined", nil,
			"Run `dx constraint add <title>` to define project boundaries"

	// ── planning ──
	case "has_features":
		if state.FeatureCount > 0 {
			return true, fmt.Sprintf("%d features", state.FeatureCount), nil, ""
		}
		return false, "no features defined", nil,
			"Run `dx feature add <name> --desc '...'` to define a demonstrable value driver"

	case "features_have_specs":
		if state.FeatureCount == 0 || state.FeaturesWithSpecs == state.FeatureCount {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d/%d features have specs", state.FeaturesWithSpecs, state.FeatureCount), nil,
			"Add specs to features: `dx spec add <feature> --kind must --desc '...'`"

	case "features_attributed":
		if state.FeatureCount == 0 || state.FeaturesAttributed == state.FeatureCount {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d/%d features linked to a goal", state.FeaturesAttributed, state.FeatureCount), nil,
			"Set goal on features: `dx feature set <name> --goal <id>`"

	case "no_untriaged_issues":
		if state.UntriagedIssues == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d untriaged issues", state.UntriagedIssues), nil, ""

	case "features_are_user_visible":
		if state.FeaturesImplDetail == 0 {
			return true, "", nil, ""
		}
		return false,
			fmt.Sprintf("%d feature(s) describe implementation details (code modules) rather than user-visible capabilities", state.FeaturesImplDetail),
			nil,
			"Features are demonstrable value drivers, not code modules — rename to describe the user benefit (e.g. 'faster search' not 'redis-cache-layer')"

	// ── verification ──
	case "specs_have_tests":
		if state.SpecsTotal == 0 || state.SpecsCovered == state.SpecsTotal {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d/%d specs covered", state.SpecsCovered, state.SpecsTotal), nil, ""

	case "goals_quantified":
		if state.GoalsTotal == 0 || state.GoalsQuantified == state.GoalsTotal {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d/%d goals have metrics", state.GoalsQuantified, state.GoalsTotal), nil,
			"Add metrics: `dx goal add <title> --metric-name <name> --metric-unit <unit>`"

	case "no_overspecced_features":
		if state.OverspeccedCount == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d features have >8 specs (decompose them)", state.OverspeccedCount), nil,
			"Split over-specced features into sub-features with `dx feature add --parent <name>`"

	// ── agents ──
	case "claude_installed":
		if state.ClaudeInstalled {
			return true, "claude CLI found", nil, ""
		}
		return false, "claude CLI not in PATH", nil, ""

	case "docker_available":
		if state.DockerAvailable {
			return true, "docker running", nil, ""
		}
		return false, "docker not available", nil, ""

	case "agent_config_set":
		if state.AgentConfigSet {
			return true, "", nil, ""
		}
		return false, "no agent config in .zdx/config.yaml", nil,
			"Edit .zdx/config.yaml to add an agent section with llm_provider and max_worktrees (run `dx config show` to view current config)"

	case "no_stale_agent_sessions":
		if state.StaleAgentSessions == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d agent sessions open past %dm idle (server auto-closes as 'orphaned'; inspect the agents UI)", state.StaleAgentSessions, state.StaleAgentSessionsMinutes), nil, ""

	case "dev_container_defined":
		switch {
		case state.HasDevDockerfile && state.HasDevCompose:
			return true, "dev.Dockerfile + docker-compose.dev.yml found", nil, ""
		case state.HasDevDockerfile:
			return false, "dev.Dockerfile found but docker-compose.dev.yml missing", func() error {
				stack := DetectStack(".")
				return os.WriteFile("docker-compose.dev.yml", []byte(RenderDevCompose(stack)), 0o644)
			}, "Run `dx doctor --fix` to scaffold docker-compose.dev.yml"
		default:
			return false, "no dev container files found", func() error {
				stack := DetectStack(".")
				if err := os.WriteFile("dev.Dockerfile", []byte(RenderDevDockerfile(stack)), 0o644); err != nil {
					return err
				}
				return os.WriteFile("docker-compose.dev.yml", []byte(RenderDevCompose(stack)), 0o644)
			}, "Run `dx doctor --fix` to scaffold dev.Dockerfile and docker-compose.dev.yml for reproducible, isolated dev environments and parallel agent runs"
		}

	// ── distribution / operations ──
	case "has_readme":
		if state.HasReadme {
			return true, "", nil, ""
		}
		return false, "no README.md", nil, "Create a README.md describing the project"

	case "has_license":
		if state.HasLicense {
			return true, "", nil, ""
		}
		return false, "no LICENSE file", nil, "Add a LICENSE file"

	case "has_changelog":
		if state.HasChangelog {
			return true, "", nil, ""
		}
		return false, "no CHANGELOG.md", nil, "Create a CHANGELOG.md"

	case "has_build_steps":
		if state.HasBuildSteps {
			return true, "", nil, ""
		}
		return false, "no build steps in config", nil, ""

	case "no_raw_api_calls":
		total := state.RawAPICallsGo + state.RawAPICallsUI
		if total == 0 {
			return true, "all API calls use typed clients", nil, ""
		}
		return false, fmt.Sprintf("%d raw API callsites (%d Go CLI, %d UI) — migrate to typed clients", total, state.RawAPICallsGo, state.RawAPICallsUI), nil, ""

	case "layered-bdd-tests":
		// Only applicable to service/saas/site with 5+ UX specs and recorded demo tests
		c := state.Classification
		if c != ClassService && c != ClassSaaS && c != ClassSite {
			return true, "not applicable for this classification", nil, ""
		}
		if state.SpecCount < 5 {
			return true, fmt.Sprintf("%d specs (need 5+)", state.SpecCount), nil, ""
		}
		if !state.DemoTestResultsExist {
			return true, "no demo-layer test results recorded yet", nil, ""
		}
		if state.UsesStepDriver {
			return true, "StepDriver pattern already in use", nil, ""
		}
		return false, fmt.Sprintf("%d specs and demo tests present, but no StepDriver pattern in test files", state.SpecCount), nil,
			"Adopt layered BDD test architecture: extract step interfaces with `Capability() DriverSet`, wire `given` steps to API driver always, route `when/then` to the selected driver. See `dx pattern show layered-bdd-tests` for the reference pattern."

	case "has_deploy_config":
		if state.ShipsFromDevDirectly {
			return false, "ships from dev/main directly to production (no gate branch)", nil,
				"Introduce a tested gate branch between dev and production for safer deploys: " +
					"create a `staging` or `gate` branch, add CI that runs your full test suite " +
					"before any merge to the deploy branch. " +
					"Use `git checkout -b staging && git push -u origin staging` to start."
		}
		return true, "deploy pipeline has a gate branch", nil, ""

	case "has_healthcheck", "has_auth", "has_tenant_isolation":
		// Placeholder — these require deeper inspection
		return true, "not yet checked", nil, ""

	default:
		return true, "unknown check", nil, ""
	}
}

// Summary counts for a set of findings.
type Summary struct {
	Total    int
	Passed   int
	Failed   int
	Deferred int
	AutoFix  int
	Propose  int
}

func Summarize(findings []Finding) Summary {
	var s Summary
	for _, f := range findings {
		s.Total++
		switch f.Status {
		case StatusPass:
			s.Passed++
		case StatusFail:
			s.Failed++
			if f.FixFunc != nil {
				s.AutoFix++
			}
			if f.Proposal != "" {
				s.Propose++
			}
		case StatusDeferred:
			s.Deferred++
		}
	}
	return s
}
