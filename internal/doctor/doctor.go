package doctor

import (
	"context"
	"fmt"
	"net/http"
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

// ForceClosedIssue describes one closed issue surfaced by the
// "force_closes_have_work_log" doctor check.
type ForceClosedIssue struct {
	ID     string // e.g. "IS-123"
	Title  string
	Reason string // e.g. "wontfix", "duplicate", "link"
}

// HistoricalOffender describes one (issue, gate) offense surfaced by the
// "closed_issues_pass_gates" retroactive-audit check (IS-632).
type HistoricalOffender struct {
	IssueID string // e.g. "IS-88"
	Gate    string // "no-worklog" | "open-tasks" | "missing-demo"
	Detail  string // e.g. failing task ID, first must-spec ID
}

// TodoQueueHealth captures aggregate todo-queue state for the queue_health
// rung (IS-956). Populated from GET /api/dx/todos/queue-health.
type TodoQueueHealth struct {
	TotalOpen             int
	BlockedCount          int
	UnblockedCount        int
	DominantBlockedReason string
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

	// KPI breach detection (from server)
	KpiBreachedChecks []string // check_names exceeding 2× trailing median or >15s absolute

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

	// Ship config coverage
	ComponentsWithoutShip int // components declared in config where Ship.IsZero()

	// Deploy pipeline
	ShipsFromDevDirectly bool   // true when project deploys from dev/main without a gate/staging branch
	DeployStrategy       string // declared deploy.strategy from .zdx/config.yaml (trunk|gate-branch|release-branch|manual|"")
	HasReleaseBranch     bool   // true when a release/* branch exists locally or on origin
	DeployEventCount     int    // total deploys recorded across all environments
	LastDeployStatus     string // "success" | "failure" | "unknown"

	// Concern coverage
	ConcernCount               int
	FeaturesWithConcerns       int
	ConcernsWithSpecsNoPattern int
	SecurityConcernSpecCount   int

	// Force-closed accountability — closed issues with a non-done close reason
	// (wontfix/duplicate/link) and zero substantive work-log entries. Captured
	// as a sample list (capped) plus a total count so the rung can summarize.
	ForceClosedNoSubstance      []ForceClosedIssue
	ForceClosedNoSubstanceTotal int

	// Retroactive close-gate audit (IS-632). Closed issues (excluding
	// force-closed and tracker/ops types) that fail one or more of the
	// IS-560 close-gate predicates: no-worklog, open-tasks, missing-demo.
	HistoricalOffenderCount   int
	HistoricalOffendersByGate map[string]int
	HistoricalOffenderSample  []HistoricalOffender // capped to first 5

	// Todo queue health — drives the queue_health rung (IS-956).
	TodoQueueHealth TodoQueueHealth
	OpenIssueCount  int // total non-closed issues; gates queue_has_claimable_work

	// Maturity questionnaire (from server)
	MaturityQuestions []dxclient.MaturityQuestion
	MaturityItems     []dxclient.MaturityItem

	// Deferred checks (from server)
	Deferred map[string]bool

	// Branching strategy (IS-967): role-flag presence drives the
	// branching_strategy_appropriate rung. Highest matched rung wins:
	//   rung4 = HasDev && HasNamedRelease
	//   rung3 = HasDev
	//   rung2 = HasPRTarget
	//   rung1 = otherwise (incl. zero rows or only rolling-release)
	BranchingStrategyRung              int
	BranchingStrategyHasDev            bool
	BranchingStrategyHasPRTarget       bool
	BranchingStrategyHasNamedRelease   bool
	BranchingStrategyHasRollingRelease bool

	// CommitterCount drives the tool/site nudge threshold; populated locally
	// from `git log --format=%ae | sort -u | wc -l`. Zero in non-git dirs.
	CommitterCount int

	// Embedder/index health (IS-811): populated from /api/health subsystems.
	// EmbedderConfigured is true when the server has a live LLM client
	// (subsystems.embedder.state == "ok"). When false, IndexBuilt and
	// EmbedderResponsive auto-pass (the rung is N/A until an LLM is wired up).
	EmbedderConfigured bool
	IndexBuilt         bool
	EmbedderResponsive bool
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
		state.DeployStrategy = cfg.ResolvedDeploy().Strategy
		for _, comp := range cfg.Components {
			if comp.Ship.IsZero() {
				state.ComponentsWithoutShip++
			}
		}
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

	// Release-branch detection: any local or remote `release/*` branch counts.
	state.HasReleaseBranch = detectReleaseBranch()

	// Distinct git committers — feeds the branching_strategy_appropriate
	// nudge threshold for tool/site projects.
	state.CommitterCount = gitCommitterCount()
}

// gitCommitterCount returns the number of distinct committer emails in the
// current repository's history. Returns 0 in non-git directories or on any
// git error so the rung degrades gracefully rather than failing the run.
func gitCommitterCount() int {
	out, err := exec.Command("git", "log", "--format=%ae").Output()
	if err != nil {
		return 0
	}
	seen := map[string]struct{}{}
	for _, line := range strings.Split(string(out), "\n") {
		e := strings.TrimSpace(line)
		if e != "" {
			seen[e] = struct{}{}
		}
	}
	return len(seen)
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

// detectReleaseBranch reports whether any branch matching `release/*` exists
// locally or on origin. Used by has_deploy_config when deploy.strategy is
// release-branch.
func detectReleaseBranch() bool {
	out, err := exec.Command("git", "branch", "-a").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		b := strings.TrimSpace(line)
		b = strings.TrimPrefix(b, "* ")
		b = strings.TrimPrefix(b, "remotes/origin/")
		if strings.HasPrefix(b, "release/") {
			return true
		}
	}
	return false
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

	case "force_closes_have_work_log":
		if state.ForceClosedNoSubstanceTotal == 0 {
			return true, "", nil, ""
		}
		const sampleLimit = 3
		var sample []string
		for i, iss := range state.ForceClosedNoSubstance {
			if i >= sampleLimit {
				break
			}
			label := iss.ID
			if iss.Reason != "" {
				label += " (" + iss.Reason + ")"
			}
			sample = append(sample, label)
		}
		msg := fmt.Sprintf("%d force-closed issue(s) with no work-log substance: %s",
			state.ForceClosedNoSubstanceTotal, strings.Join(sample, ", "))
		if state.ForceClosedNoSubstanceTotal > sampleLimit {
			msg += fmt.Sprintf(" (+%d more)", state.ForceClosedNoSubstanceTotal-sampleLimit)
		}
		return false, msg, nil, ""

	case "closed_issues_pass_gates":
		if state.HistoricalOffenderCount == 0 {
			return true, "", nil, ""
		}
		noWorklog := state.HistoricalOffendersByGate["no-worklog"]
		openTasks := state.HistoricalOffendersByGate["open-tasks"]
		missingDemo := state.HistoricalOffendersByGate["missing-demo"]
		msg := fmt.Sprintf("%d historical close(s) fail the close-gate (%d no-worklog, %d open-tasks, %d missing-demo)",
			state.HistoricalOffenderCount, noWorklog, openTasks, missingDemo)
		if len(state.HistoricalOffenderSample) > 0 {
			first := state.HistoricalOffenderSample[0]
			msg += fmt.Sprintf(" — first: %s (%s)", first.IssueID, first.Gate)
		}
		return false, msg, nil, ""

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
			"Add metrics: `dx goal add <title> --metric-name <name> --metric-unit <unit>`. Goals should be outcomes (users can X), not engineering domains (correctness, performance)."

	case "no_overspecced_features":
		if state.OverspeccedCount == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d features have >8 must/should specs (decompose them)", state.OverspeccedCount), nil,
			"Split over-specced features into sub-features with `dx feature add --parent <name>`"

	case "kpi_check_breaches":
		if len(state.KpiBreachedChecks) == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d check(s) breached: %s", len(state.KpiBreachedChecks), strings.Join(state.KpiBreachedChecks, ", ")), nil, ""

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

	// ── embedder_health (IS-811) ──
	case "embedder_configured":
		if state.EmbedderConfigured {
			return true, "embedder subsystem ok", nil, ""
		}
		return false, "embedder subsystem not configured (no live LLM client)", nil,
			"Run `dx config llm add --provider <name> --model <id>` to wire an LLM provider, then restart dx-server"

	case "index_built":
		if !state.EmbedderConfigured {
			return true, "embedder unconfigured — index check N/A", nil, ""
		}
		if state.IndexBuilt {
			return true, "index files present", nil, ""
		}
		return false, "index subsystem reports no zvec files yet", nil,
			"Restart dx-server to trigger ReindexAllIssues, or run `dx reindex` once available"

	case "embedder_responsive":
		if !state.EmbedderConfigured || state.EmbedderResponsive {
			return true, "", nil, ""
		}
		return false, "embedder reports a recent failure (subsystems.embedder.reason set)", nil, ""

	// ── queue_health (IS-956) ──
	case "queue_has_claimable_work":
		if state.OpenIssueCount > 0 && state.TodoQueueHealth.TotalOpen > 0 && state.TodoQueueHealth.UnblockedCount == 0 {
			reason := state.TodoQueueHealth.DominantBlockedReason
			if reason == "" {
				reason = "unknown"
			}
			return false,
				fmt.Sprintf("%d/%d open todos blocked; dominant reason: %q",
					state.TodoQueueHealth.BlockedCount, state.TodoQueueHealth.TotalOpen, reason),
				nil,
				"File a system-gap issue describing why all todos are blocked, or run `dx todo unblock-all` if the blocks are stale"
		}
		return true, "queue has claimable work", nil, ""

	case "queue_blocked_ratio_ok":
		if state.TodoQueueHealth.TotalOpen > 0 {
			ratio := float64(state.TodoQueueHealth.BlockedCount) / float64(state.TodoQueueHealth.TotalOpen)
			if ratio > 0.8 {
				return false,
					fmt.Sprintf("%.0f%% of open todos blocked (%d/%d)",
						ratio*100, state.TodoQueueHealth.BlockedCount, state.TodoQueueHealth.TotalOpen),
					nil,
					"Investigate and resolve the dominant blocked reason; run `dx todo unblock-all` if blocks are stale"
			}
		}
		return true, "blocked ratio within threshold", nil, ""

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

	case "ship_config_defined":
		if state.Config == nil || len(state.Config.Components) == 0 {
			return true, "no components declared", nil, ""
		}
		if state.ComponentsWithoutShip == 0 {
			return true, "all components have ship config", nil, ""
		}
		total := len(state.Config.Components)
		return false, fmt.Sprintf("%d/%d components missing ship config", state.ComponentsWithoutShip, total), nil,
			"add ship: section to .zdx/config.yaml for each component"

	case "has_deploy_config":
		switch state.DeployStrategy {
		case config.DeployStrategyTrunk:
			return true, "trunk deploy declared (rolling/blue-green safety)", nil, ""
		case config.DeployStrategyGateBranch:
			if state.ShipsFromDevDirectly {
				return false, "ships from dev/main directly to production (no gate branch)", nil,
					"Introduce a tested gate branch between dev and production for safer deploys: " +
						"create a `staging` or `gate` branch, add CI that runs your full test suite " +
						"before any merge to the deploy branch. " +
						"Use `git checkout -b staging && git push -u origin staging` to start."
			}
			return true, "gate branch present", nil, ""
		case config.DeployStrategyReleaseBranch:
			if !state.HasReleaseBranch {
				return false, "release-branch strategy declared but no release/* branch found", nil,
					"Create a release branch to mark the next shippable cut: " +
						"`git checkout -b release/v0 && git push -u origin release/v0`."
			}
			return true, "release/* branch present", nil, ""
		case config.DeployStrategyManual:
			return true, "manual deploy strategy declared", nil, ""
		case "":
			// Unset: preserve current behavior (default = gate-branch detection)
			// so existing projects don't silently flip.
			if state.ShipsFromDevDirectly {
				return false, "ships from dev/main directly to production (no gate branch)", nil,
					"Introduce a tested gate branch between dev and production for safer deploys: " +
						"create a `staging` or `gate` branch, add CI that runs your full test suite " +
						"before any merge to the deploy branch. " +
						"Use `git checkout -b staging && git push -u origin staging` to start."
			}
			return true, "deploy pipeline has a gate branch", nil, ""
		default:
			return false, fmt.Sprintf("deploy.strategy=%s not recognized; expected trunk|gate-branch|release-branch|manual", state.DeployStrategy), nil,
				"Set deploy.strategy in .zdx/config.yaml to one of: trunk, gate-branch, release-branch, manual."
		}

	case "has_deploy_events":
		if state.DeployEventCount > 0 {
			return true, fmt.Sprintf("%d deploys recorded", state.DeployEventCount), nil, ""
		}
		return false, "no deploys recorded — run bin/ship or dx ship run to record one", nil, ""

	case "last_deploy_verify_passed":
		switch state.LastDeployStatus {
		case "success", "unknown", "":
			return true, "", nil, ""
		default:
			return false, fmt.Sprintf("last deploy status: %s", state.LastDeployStatus), nil, ""
		}

	// ── concerns ──
	case "concerns_defined":
		if state.ConcernCount > 0 {
			return true, fmt.Sprintf("%d concerns", state.ConcernCount), nil, ""
		}
		return false, "no concerns defined", func() error {
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no config loaded")
			}
			slug := cfg.RemoteSlug()
			apiKey := config.RemoteAPIKey()
			c, err := dxclient.NewClientWithResponses(cfg.RemoteURL(),
				dxclient.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
					if apiKey != "" {
						req.Header.Set("Authorization", "Bearer "+apiKey)
					}
					return nil
				}),
			)
			if err != nil {
				return err
			}
			defaults := []string{"Security", "Latency", "Compatibility", "Operability", "Accessibility", "UX", "Functional"}
			for _, name := range defaults {
				if _, err := c.CreateConcernWithResponse(context.Background(), dxclient.CreateConcernJSONRequestBody{
					Slug: slug, Name: name,
				}); err != nil {
					return fmt.Errorf("create concern %q: %w", name, err)
				}
			}
			return nil
		}, "Run `dx doctor --fix` to seed default concerns, or `dx concern add --name Security`"

	case "features_have_concerns":
		if state.FeaturesWithSpecs == 0 || state.FeaturesWithConcerns == state.FeaturesWithSpecs {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d/%d specced features have concern attribution", state.FeaturesWithConcerns, state.FeaturesWithSpecs), nil,
			"Link features to concerns: `dx concern link --concern Security --feature <name>`"

	case "concerns_have_patterns":
		if state.ConcernsWithSpecsNoPattern == 0 {
			return true, "", nil, ""
		}
		return false, fmt.Sprintf("%d concern(s) have specs but no attributed pattern", state.ConcernsWithSpecsNoPattern), nil,
			"Add pattern attribution: `dx concern link --concern <name> --pattern <id>`"

	case "security_concern_specs":
		if state.SecurityConcernSpecCount > 0 {
			return true, fmt.Sprintf("%d security specs", state.SecurityConcernSpecCount), nil, ""
		}
		if state.ConcernCount == 0 {
			return true, "no concerns defined yet", nil, ""
		}
		return false, "Security concern has no specs", nil,
			"Add a security spec: `dx spec add <feature> --kind must --concern Security --desc '...'`"

	case "branching_strategy_appropriate":
		return runBranchingStrategyCheck(state)

	case "has_healthcheck", "has_auth", "has_tenant_isolation":
		// Placeholder — these require deeper inspection
		return true, "not yet checked", nil, ""

	default:
		return true, "unknown check", nil, ""
	}
}

// computeBranchingRung returns the highest matched rung (1–4) for the role
// flags in state. Order matters: rung4 wins over rung3 wins over rung2.
func computeBranchingRung(state *ProjectState) int {
	switch {
	case state.BranchingStrategyHasDev && state.BranchingStrategyHasNamedRelease:
		return 4
	case state.BranchingStrategyHasDev:
		return 3
	case state.BranchingStrategyHasPRTarget:
		return 2
	default:
		return 1
	}
}

// runBranchingStrategyCheck evaluates the branching_strategy_appropriate rung.
// Library passes at any rung. Service/SaaS at rung 1 fail with a nudge to
// rung 3. Tool/Site at rung 1 with prior deploys fail with a nudge to rung 2.
// Everything else passes. The proposal text names the classification, current
// rung, target rung, and the concrete CLI command to advance.
func runBranchingStrategyCheck(state *ProjectState) (pass bool, msg string, fixFunc func() error, proposal string) {
	rung := computeBranchingRung(state)
	state.BranchingStrategyRung = rung

	class := state.Classification
	if class == "" {
		class = ClassTool
	}

	rungLabel := fmt.Sprintf("rung %d", rung)
	if rung == 1 {
		rungLabel = "rung 1 (no version-branch rows)"
	}

	switch class {
	case ClassLibrary:
		return true, fmt.Sprintf("library at %s — branching strategy not gated", rungLabel), nil, ""

	case ClassService, ClassSaaS:
		if rung == 1 {
			return false,
				fmt.Sprintf("%s at rung 1 — no dev branch row; deploys lack a buffer", class),
				nil,
				"Advance to rung 3: add a dev branch and wire main.source_branch_name=dev for a deploy buffer. " +
					"Run `dx branch add-role dev --name dev` and `dx branch set-source main --source dev`."
		}
		return true, fmt.Sprintf("%s at %s", class, rungLabel), nil, ""

	case ClassTool, ClassSite:
		if rung == 1 && state.DeployEventCount > 0 {
			return false,
				fmt.Sprintf("%s at rung 1 with %d recorded deploy(s) — no pr-target buffer for review", class, state.DeployEventCount),
				nil,
				"Advance to rung 2: introduce a pr-target branch so changes can be reviewed before they ship. " +
					"Run `dx branch add-role pr-target --name <branch>`."
		}
		return true, fmt.Sprintf("%s at %s", class, rungLabel), nil, ""

	default:
		return true, fmt.Sprintf("%s at %s", class, rungLabel), nil, ""
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
