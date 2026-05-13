package agent

import (
	"encoding/json"
	"testing"
)

// runBashArgsJSON builds the JSON the LLM would pass for a run_bash tool
// call. Tests reuse this so a typo in the field name fails one place, not N.
func runBashArgsJSON(command string) string {
	b, _ := json.Marshal(map[string]any{"command": command})
	return string(b)
}

// TestPermissionMatrix walks every (persona, tool, args) combination spelled
// out in IS-1097 and asserts the allow/deny decision matches the design
// matrix. The combination list is intentionally explicit — failure should
// point at exactly which cell drifted.
func TestPermissionMatrix(t *testing.T) {
	type tc struct {
		name        string
		persona     string
		tool        string
		args        string
		wantAllowed bool
		wantTier    string // when wantAllowed=false
	}

	cases := []tc{
		// ── edit_file / write_file ────────────────────────────────────
		{name: "dev/edit_file", persona: PersonaDev, tool: "edit_file", wantAllowed: true},
		{name: "tech/edit_file", persona: PersonaTech, tool: "edit_file", wantAllowed: true},
		{name: "reviewer/edit_file", persona: PersonaReviewer, tool: "edit_file", wantTier: PersonaTech},
		{name: "product/edit_file", persona: PersonaProduct, tool: "edit_file", wantTier: PersonaTech},

		{name: "dev/write_file", persona: PersonaDev, tool: "write_file", wantAllowed: true},
		{name: "tech/write_file", persona: PersonaTech, tool: "write_file", wantAllowed: true},
		{name: "reviewer/write_file", persona: PersonaReviewer, tool: "write_file", wantTier: PersonaTech},
		{name: "product/write_file", persona: PersonaProduct, tool: "write_file", wantTier: PersonaTech},

		// ── read-only file/inspection tools always allowed ────────────
		{name: "reviewer/read_file", persona: PersonaReviewer, tool: "read_file", wantAllowed: true},
		{name: "product/read_file", persona: PersonaProduct, tool: "read_file", wantAllowed: true},
		{name: "reviewer/grep", persona: PersonaReviewer, tool: "grep", wantAllowed: true},
		{name: "product/glob", persona: PersonaProduct, tool: "glob", wantAllowed: true},
		{name: "reviewer/list_dir", persona: PersonaReviewer, tool: "list_dir", wantAllowed: true},
		{name: "product/outline", persona: PersonaProduct, tool: "outline", wantAllowed: true},

		// ── run_bash arbitrary: dev/tech allowed, reviewer/product denied
		{name: "dev/run_bash arbitrary", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("ls -la"), wantAllowed: true},
		{name: "tech/run_bash arbitrary", persona: PersonaTech, tool: "run_bash",
			args: runBashArgsJSON("ls -la"), wantAllowed: true},
		{name: "reviewer/run_bash arbitrary", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("ls -la"), wantTier: PersonaTech},
		{name: "product/run_bash arbitrary", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("ls -la"), wantTier: PersonaTech},

		// ── run_bash dx subset: allowed for reviewer & product ───────
		{name: "reviewer/run_bash dx", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue list"), wantAllowed: true},
		{name: "reviewer/run_bash binDx", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("bin/dx issue show IS-1"), wantAllowed: true},
		{name: "reviewer/run_bash /workspace path", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("/workspace/bin/dx issue list"), wantAllowed: true},
		{name: "product/run_bash dx", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx focus list"), wantAllowed: true},

		// ── dx subset must not be a chained shell ────────────────────
		{name: "reviewer/run_bash dx chained", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue list && rm -rf /tmp/foo"), wantTier: PersonaTech},
		{name: "reviewer/run_bash dx semi", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue list ; ls"), wantTier: PersonaTech},
		{name: "reviewer/run_bash dx pipe", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue list | grep foo"), wantTier: PersonaTech},
		{name: "reviewer/run_bash dx redir", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue list > /tmp/out"), wantTier: PersonaTech},
		{name: "reviewer/run_bash dx substitution", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("echo $(dx issue list)"), wantTier: PersonaTech},
		{name: "reviewer/run_bash dx backtick", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("echo `dx issue list`"), wantTier: PersonaTech},

		// ── dx feature add/edit: product only ────────────────────────
		{name: "dev/dx feature add", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("dx feature add FT-1 --title X"), wantTier: PersonaProduct},
		{name: "tech/dx feature edit", persona: PersonaTech, tool: "run_bash",
			args: runBashArgsJSON("dx feature edit FT-1 --title Y"), wantTier: PersonaProduct},
		{name: "reviewer/dx feature add", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx feature add FT-1 --title Z"), wantTier: PersonaProduct},
		{name: "product/dx feature add", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx feature add FT-1 --title Q"), wantAllowed: true},
		{name: "product/dx feature edit", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx feature edit FT-1 --title R"), wantAllowed: true},

		// ── dx goal add/edit: product only ───────────────────────────
		{name: "dev/dx goal add", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("dx goal add G-1 --title A"), wantTier: PersonaProduct},
		{name: "tech/dx goal edit", persona: PersonaTech, tool: "run_bash",
			args: runBashArgsJSON("dx goal edit G-1 --title B"), wantTier: PersonaProduct},
		{name: "product/dx goal add", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx goal add G-1 --title C"), wantAllowed: true},

		// ── dx gate-item meet: reviewer only ─────────────────────────
		{name: "dev/dx gate-item meet", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item meet GI-1"), wantTier: PersonaReviewer},
		{name: "tech/dx gate-item meet", persona: PersonaTech, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item meet GI-1"), wantTier: PersonaReviewer},
		{name: "product/dx gate-item meet", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item meet GI-1"), wantTier: PersonaReviewer},
		{name: "reviewer/dx gate-item meet", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item meet GI-1"), wantAllowed: true},

		// ── dx gate-item waive: reviewer & product ───────────────────
		{name: "dev/dx gate-item waive", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item waive GI-1"), wantTier: PersonaReviewer},
		{name: "tech/dx gate-item waive", persona: PersonaTech, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item waive GI-1"), wantTier: PersonaReviewer},
		{name: "reviewer/dx gate-item waive", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item waive GI-1"), wantAllowed: true},
		{name: "product/dx gate-item waive", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx gate-item waive GI-1"), wantAllowed: true},

		// ── universally-allowed dx subcommands ───────────────────────
		{name: "reviewer/dx issue add", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue add --title X"), wantAllowed: true},
		{name: "product/dx question add", persona: PersonaProduct, tool: "run_bash",
			args: runBashArgsJSON("dx question add --route=user"), wantAllowed: true},
		{name: "dev/dx todo take", persona: PersonaDev, tool: "run_bash",
			args: runBashArgsJSON("dx todo take"), wantAllowed: true},
		{name: "reviewer/dx issue close", persona: PersonaReviewer, tool: "run_bash",
			args: runBashArgsJSON("dx issue close IS-1"), wantAllowed: true},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CheckToolPermission(c.persona, c.tool, c.args)
			if got.Allowed != c.wantAllowed {
				t.Errorf("CheckToolPermission allowed=%v, want %v (decision=%+v)",
					got.Allowed, c.wantAllowed, got)
				return
			}
			if !c.wantAllowed && got.SuggestedTier != c.wantTier {
				t.Errorf("suggested tier = %q, want %q (decision=%+v)",
					got.SuggestedTier, c.wantTier, got)
			}
		})
	}
}

// TestPermissionMatrixUnknownPersona ensures unknown personas fall back to
// the default (dev) tier permissions rather than panicking or silently
// allowing everything.
func TestPermissionMatrixUnknownPersona(t *testing.T) {
	got := CheckToolPermission("bogus", "edit_file", "")
	if !got.Allowed {
		t.Fatalf("unknown persona should fall back to dev (allow edit_file), got %+v", got)
	}
}

// TestFormatToolPermissionError pins the canonical wire format from IS-1097
// so the LLM-facing message stays stable for prompt-engineering work.
func TestFormatToolPermissionError(t *testing.T) {
	msg := FormatToolPermissionError(PersonaReviewer, "edit_file", ToolPermissionDecision{
		SuggestedTier:   PersonaTech,
		SuggestedAction: "implementation",
	})
	want := "persona=reviewer cannot edit_file; route a question to tech if implementation is needed."
	if msg != want {
		t.Errorf("format = %q, want %q", msg, want)
	}
}

// TestRunBashEmptyAndMalformedArgs makes sure the parser tolerates whatever
// the LLM throws at it instead of crashing the dispatch loop.
func TestRunBashEmptyAndMalformedArgs(t *testing.T) {
	// reviewer + empty args → no command → not classified as dx →
	// arbitrary-shell deny.
	d := CheckToolPermission(PersonaReviewer, "run_bash", "")
	if d.Allowed {
		t.Errorf("reviewer with empty run_bash args should be denied, got %+v", d)
	}
	if d.SuggestedTier != PersonaTech {
		t.Errorf("suggested tier = %q, want tech", d.SuggestedTier)
	}

	// Malformed JSON: same outcome — treat as missing command.
	d = CheckToolPermission(PersonaReviewer, "run_bash", "{not-json")
	if d.Allowed {
		t.Errorf("reviewer with malformed args should be denied, got %+v", d)
	}

	// dev with the same malformed args is still allowed: arbitrary-shell
	// permission applies regardless of whether we could classify it.
	d = CheckToolPermission(PersonaDev, "run_bash", "{not-json")
	if !d.Allowed {
		t.Errorf("dev with malformed args should still be allowed (arbitrary shell), got %+v", d)
	}
}
