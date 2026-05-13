package agent

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ToolPermissionDecision is the result of checking whether a (persona, tool,
// args) tuple is allowed at the MCP dispatch boundary. Allowed=true means the
// dispatcher should forward to CallTool. Allowed=false means the dispatcher
// should short-circuit and return Reason to the agent as a tool-result error.
//
// SuggestedTier is the tier the agent should consider routing a question to
// (via dx question add --route=<tier>) if the underlying action is needed.
// SuggestedAction is a short verb phrase describing the denied capability,
// used to render the canonical error string from IS-1097.
type ToolPermissionDecision struct {
	Allowed         bool
	SuggestedTier   string
	SuggestedAction string
}

// FormatToolPermissionError renders the canonical banned-tool message from
// IS-1097: "persona=<P> cannot <tool>; route a question to <tier> if <action>
// is needed." Callers pass the same tool name the LLM invoked so the message
// is unambiguous when echoed back through the tool-result channel.
func FormatToolPermissionError(persona, tool string, d ToolPermissionDecision) string {
	return fmt.Sprintf(
		"persona=%s cannot %s; route a question to %s if %s is needed.",
		persona, tool, d.SuggestedTier, d.SuggestedAction,
	)
}

// CheckToolPermission applies the IS-1090 design matrix at the tool-dispatch
// boundary. Reads persona from the caller (the agent session's role tier),
// inspects the tool name and (for run_bash) the parsed command, and returns
// Allowed=false with a structured suggestion when the persona is not
// permitted to invoke the tool.
//
// Matrix coverage (see IS-1097):
//   - edit_file / write_file: dev, tech allowed; reviewer, product denied.
//   - run_bash arbitrary:     dev, tech allowed; reviewer, product denied
//     unless the command is recognizably a dx CLI invocation (no shell
//     chaining / IO redirection / command substitution).
//   - dx subcommand gating:   feature add/edit and goal add/edit are product
//     only; gate-item meet is reviewer only; gate-item waive is reviewer or
//     product. dx issue close, dx issue add, dx question add are allowed for
//     all four personas (ownership and route-graph validation are server-
//     side concerns).
//
// Read-only tools (read_file, list_dir, glob, grep, outline) are allowed for
// every persona — the filter explicitly never denies them so reviewer and
// product can still inspect the tree to do their job.
func CheckToolPermission(persona, tool, argsJSON string) ToolPermissionDecision {
	p, err := NormalizePersona(persona)
	if err != nil {
		p = DefaultPersona
	}

	switch tool {
	case "edit_file", "write_file":
		if p == PersonaReviewer || p == PersonaProduct {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaTech,
				SuggestedAction: "implementation",
			}
		}
		return ToolPermissionDecision{Allowed: true}

	case "run_bash":
		return checkRunBashPermission(p, argsJSON)

	default:
		// Read-only / inspection tools (read_file, list_dir, glob, grep,
		// outline) and anything not in the matrix default-allow. The matrix
		// is an allowlist for *forbidden* actions; tools without a row are
		// neutral and available to every persona.
		return ToolPermissionDecision{Allowed: true}
	}
}

// runBashArgs mirrors the input struct of mcpcmd.RegisterShellTools so we can
// peek at the Command field without depending on the mcpcmd package.
type runBashArgs struct {
	Command string `json:"command"`
}

func checkRunBashPermission(persona, argsJSON string) ToolPermissionDecision {
	cmd := parseRunBashCommand(argsJSON)

	dxVerb, isDX := classifyDXCommand(cmd)

	// reviewer / product can only run_bash if the command is a single dx-CLI
	// invocation. Arbitrary shell is denied at the tool boundary so the LLM
	// gets a structured error instead of silently executing rm/cat/etc.
	if persona == PersonaReviewer || persona == PersonaProduct {
		if !isDX {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaTech,
				SuggestedAction: "arbitrary shell access",
			}
		}
	}

	if isDX {
		if d := checkDXSubcommandPermission(persona, dxVerb); !d.Allowed {
			return d
		}
	}

	return ToolPermissionDecision{Allowed: true}
}

func parseRunBashCommand(argsJSON string) string {
	if argsJSON == "" {
		return ""
	}
	var a runBashArgs
	if err := json.Unmarshal([]byte(argsJSON), &a); err != nil {
		return ""
	}
	return a.Command
}

// classifyDXCommand returns the dx subcommand verb (e.g. "feature add",
// "gate-item meet") when cmd is a single, unchained invocation of the dx
// binary. Returns isDX=false when the command contains shell metacharacters
// that could chain or redirect — even if the first token is dx — to keep the
// dx-subset gate from being trivially bypassed by `dx ls && rm -rf /`.
func classifyDXCommand(cmd string) (verb string, isDX bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", false
	}

	// Bash metacharacters that introduce chaining, piping, redirection, or
	// command substitution. Anything containing these is treated as
	// arbitrary shell, regardless of the leading token.
	const shellMeta = ";&|<>`$()"
	if strings.ContainsAny(trimmed, shellMeta) {
		return "", false
	}
	// Newlines and backslash-continuations also break the single-command
	// assumption — reject them explicitly.
	if strings.ContainsAny(trimmed, "\n\r\\") {
		return "", false
	}

	fields := strings.Fields(trimmed)
	if len(fields) == 0 {
		return "", false
	}
	if !isDXBinary(fields[0]) {
		return "", false
	}

	// Collect non-flag positional tokens up to the first flag so we can
	// classify multi-word verbs like "feature add" or "gate-item meet"
	// without being fooled by global flags.
	verbs := make([]string, 0, 2)
	for _, f := range fields[1:] {
		if strings.HasPrefix(f, "-") {
			break
		}
		verbs = append(verbs, f)
		if len(verbs) >= 2 {
			break
		}
	}
	return strings.Join(verbs, " "), true
}

// isDXBinary reports whether tok refers to the dx CLI. Accepts the bare name
// (PATH lookup), repo-relative bin/dx, and the in-container absolute path
// /workspace/bin/dx (per IS-1112).
func isDXBinary(tok string) bool {
	switch tok {
	case "dx", "bin/dx", "./bin/dx", "/workspace/bin/dx":
		return true
	}
	return false
}

// checkDXSubcommandPermission applies the dx CLI subcommand restrictions from
// the IS-1090 matrix. verb is a 1- or 2-word dx subcommand path (e.g.
// "feature add", "gate-item meet"). Returns Allowed=true for any subcommand
// not specifically gated.
func checkDXSubcommandPermission(persona, verb string) ToolPermissionDecision {
	switch verb {
	case "feature add", "feature edit", "feature set":
		if persona != PersonaProduct {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaProduct,
				SuggestedAction: "a feature change",
			}
		}
	case "goal add", "goal edit", "goal set":
		if persona != PersonaProduct {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaProduct,
				SuggestedAction: "a goal change",
			}
		}
	case "gate-item meet":
		if persona != PersonaReviewer {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaReviewer,
				SuggestedAction: "marking a gate item met",
			}
		}
	case "gate-item waive":
		if persona != PersonaReviewer && persona != PersonaProduct {
			return ToolPermissionDecision{
				SuggestedTier:   PersonaReviewer,
				SuggestedAction: "waiving a gate item",
			}
		}
	}
	return ToolPermissionDecision{Allowed: true}
}
