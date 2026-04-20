// Package doctor implements project health checks organized as maturity vines.
// Each project classification (library, tool, service, saas, site) has a vine —
// an ordered set of rungs, each with checks. Doctor runs checks, auto-fixes what
// it can, and proposes the rest.
package doctor

// Classification is a project type that shapes the maturity vine.
type Classification string

const (
	ClassLibrary Classification = "library"
	ClassTool    Classification = "tool"
	ClassService Classification = "service"
	ClassSaaS    Classification = "saas"
	ClassSite    Classification = "site"
)

var AllClassifications = []Classification{
	ClassLibrary, ClassTool, ClassService, ClassSaaS, ClassSite,
}

func ClassificationLabel(c Classification) string {
	switch c {
	case ClassLibrary:
		return "Library — reusable package, published for others"
	case ClassTool:
		return "Tool — standalone CLI or local application"
	case ClassService:
		return "Service — deployed backend, single-tenant"
	case ClassSaaS:
		return "SaaS — multi-tenant deployed service"
	case ClassSite:
		return "Site — static site, blog, or content project"
	default:
		return string(c)
	}
}

// Rung is one level on the maturity vine.
type Rung struct {
	Name        string
	Description string
	Checks      []Check
}

// CheckAction describes what doctor can do about a failing check.
type CheckAction int

const (
	ActionAutoFix CheckAction = iota // Doctor can fix this silently
	ActionPropose                    // Doctor explains the gap and proposes a fix
	ActionInfo                       // Informational — no action, just status
)

// Check is a single health check on a maturity rung.
type Check struct {
	Name        string      // unique key, e.g. "has_goals"
	Description string      // human-readable: "Project has at least one goal defined"
	Action      CheckAction // what doctor does when this fails
}

// Vine returns the maturity rungs for a classification.
// All classifications share a common base; classification-specific checks extend it.
func Vine(c Classification) []Rung {
	vine := commonVine()
	switch c {
	case ClassLibrary:
		vine = append(vine, libraryRungs()...)
	case ClassTool:
		vine = append(vine, toolRungs()...)
	case ClassService:
		vine = append(vine, serviceRungs()...)
	case ClassSaaS:
		vine = append(vine, saasRungs()...)
	case ClassSite:
		vine = append(vine, siteRungs()...)
	}
	return vine
}

// ── Common vine (all classifications) ────────────────────────────────────────

func commonVine() []Rung {
	return []Rung{
		{
			Name:        "scaffold",
			Description: "Project scaffolding — .zdx/ exists, config is valid",
			Checks: []Check{
				{"zdx_dir_exists", ".zdx/ directory exists", ActionAutoFix},
				{"config_valid", ".zdx/config.yaml is parseable", ActionAutoFix},
				{"credentials_exist", ".zdx/credentials has an API key", ActionPropose},
				{"remote_reachable", "Remote server is reachable", ActionInfo},
			},
		},
		{
			Name:        "identity",
			Description: "Project knows what it is — classification set, goals defined",
			Checks: []Check{
				{"classification_set", "Project classification is set", ActionPropose},
				{"has_goals", "At least one goal defined", ActionPropose},
				{"has_constraints", "At least one constraint defined", ActionPropose},
			},
		},
		{
			Name:        "planning",
			Description: "Work is structured — features with specs, issues triaged",
			Checks: []Check{
				{"has_features", "At least one feature defined", ActionPropose},
				{"features_have_specs", "All features have at least one spec", ActionPropose},
				{"features_attributed", "All features linked to a goal", ActionPropose},
				{"no_untriaged_issues", "No open issues without priority", ActionInfo},
			},
		},
		{
			Name:        "verification",
			Description: "Code is tested — specs have test refs, tests pass",
			Checks: []Check{
				{"specs_have_tests", "Non-deferred specs have test coverage", ActionInfo},
				{"goals_quantified", "All goals have metric_name set", ActionPropose},
				{"no_overspecced_features", "No features with >8 specs (decomposition signal)", ActionPropose},
				{"no_raw_api_calls", "CLI and UI use typed API clients (no raw URL callsites)", ActionInfo},
				{"layered-bdd-tests", "Projects with 5+ UX specs should use layered BDD tests with swappable API/UI drivers", ActionPropose},
			},
		},
		{
			Name:        "agents",
			Description: "Agent infrastructure — LLM configured, agent can run",
			Checks: []Check{
				{"claude_installed", "Claude CLI is in PATH", ActionInfo},
				{"docker_available", "Docker daemon is running", ActionInfo},
				{"agent_config_set", "Agent section in .zdx/config.yaml", ActionPropose},
				{"no_stale_agent_sessions", "No orphaned agent sessions on the remote", ActionInfo},
			},
		},
	}
}

// ── Classification-specific extensions ───────────────────────────────────────

func libraryRungs() []Rung {
	return []Rung{
		{
			Name:        "distribution",
			Description: "Library is publishable — docs, versioning, changelog",
			Checks: []Check{
				{"has_readme", "README.md exists", ActionPropose},
				{"has_license", "LICENSE file exists", ActionPropose},
				{"has_changelog", "CHANGELOG.md exists", ActionPropose},
			},
		},
	}
}

func toolRungs() []Rung {
	return []Rung{
		{
			Name:        "distribution",
			Description: "Tool is installable — build, release binary",
			Checks: []Check{
				{"has_build_steps", "Build steps configured in .zdx/config.yaml", ActionInfo},
				{"has_readme", "README.md exists", ActionPropose},
			},
		},
	}
}

func serviceRungs() []Rung {
	return []Rung{
		{
			Name:        "operations",
			Description: "Service is deployable — CI, deploy pipeline, monitoring",
			Checks: []Check{
				{"has_build_steps", "Build steps configured", ActionInfo},
				{"has_deploy_config", "Deploy pipeline configured", ActionPropose},
				{"has_healthcheck", "Health check endpoint exists", ActionInfo},
			},
		},
	}
}

func saasRungs() []Rung {
	return []Rung{
		{
			Name:        "operations",
			Description: "Service is deployable",
			Checks: []Check{
				{"has_build_steps", "Build steps configured", ActionInfo},
				{"has_deploy_config", "Deploy pipeline configured", ActionPropose},
				{"has_healthcheck", "Health check endpoint exists", ActionInfo},
			},
		},
		{
			Name:        "multi-tenancy",
			Description: "Tenant isolation and compliance",
			Checks: []Check{
				{"has_auth", "Authentication configured", ActionPropose},
				{"has_tenant_isolation", "Tenant data isolation documented", ActionPropose},
			},
		},
	}
}

func siteRungs() []Rung {
	return []Rung{
		{
			Name:        "publication",
			Description: "Site is publishable — build, deploy",
			Checks: []Check{
				{"has_build_steps", "Build steps configured", ActionInfo},
				{"has_deploy_config", "Deploy pipeline configured", ActionPropose},
			},
		},
	}
}
