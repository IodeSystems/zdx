// Package workflowhints centralizes the workflow-guidance strings surfaced to
// LLM agents and CLI users. Keeping hints in one place means that changing a
// workflow rule (e.g. how to handle similar issues, or what counts as a valid
// test plan) updates every surface — solo queue candidates, MCP tool
// responses, and CLI output — in lock-step.
//
// Every *Text builder returns self-contained agent-actionable text. An LLM
// reading only that text (no skill file, no external docs) should know
// exactly what to do for that kind.
package workflowhints

import "fmt"

// Hint is the structured return type for solo-queue candidate builders.
// Title and Description are persisted in zdx_todos; Instructions are generated
// at read time and not stored.
type Hint struct {
	Title        string // short headline for list rows and headers
	Description  string // context/why for understanding
	Instructions string // agent playbook — computed, not stored
}

// ── Shared fragments ───────────────────────────────────────────────────────
// Concatenated into builders where relevant. Keeping them as constants means
// updating the close-and-ship sequence (or the blocker-question discipline)
// updates every todo text in lock-step.

// CloseAndShipFragment describes what to do after closing an issue when the
// vertical produced shippable code. Appended to closable-style texts.
const CloseAndShipFragment = "\n\nShip if the vertical touched production code (server, UI, schema, queries): " +
	"(1) commit all changes — `git add <files> && git commit -m '...'`; " +
	"(2) if internal/migrate/sql/ or queries/*.sql changed, run `~/go/bin/sqlc generate && go build ./...` to verify; " +
	"(3) `./bin/ship` (never --allow-dirty — emergency only). " +
	"Skip ship for docs/skill/planning-only changes — just commit."

// BlockerQuestionCriteriaFragment describes when to file a `dx question add`
// instead of pushing through. Appended to kinds where judgment calls are
// likely.
const BlockerQuestionCriteriaFragment = "\n\nIf genuinely blocked by a judgment call (product direction, priority, " +
	"user-facing wording, business rules not derivable from code): " +
	"`dx question add --target-type=<issue|task|feature> --target-id=<ID> --context=\"<question>\" --choices=\"opt1,opt2,...\"`. " +
	"Exhaust code/data/docs/experiment first — only file when human input is truly required."

// PostWorkDXAnalysisFragment reminds the agent to log friction points after
// finishing a vertical. Appended to terminal kinds (closable, close:tracker).
const PostWorkDXAnalysisFragment = "\n\nPost-work: if you flailed on a command, hit an unclear error, or had to " +
	"discover what should have been obvious — file a DX issue: " +
	"`dx issue add --title=\"...\" --context=\"<what happened / what the tool should have said instead>\"`. " +
	"Do not file for expected complexity or user error — only when the tool failed to guide correctly."

// StopAfterVerticalFragment is the terminal cue: one vertical per session.
const StopAfterVerticalFragment = "\n\nOne vertical per session — stop after this issue is closed and shipped."

// TriageChecklist is the checklist block printed after the triage context
// (active focuses/goals + similar issues) in `dx todo solo`.
const TriageChecklist = `  triage checklist:
    1. verify independently (reproduce or read the code)
    2. dup-check: scan the 'similar issues' list above (and dx issue list for wider context).
       - full duplicate (same bug/ask): dx issue close IS-N --reason=duplicate --duplicate-of=IS-X
       - narrow slice of a larger issue: dx issue close IS-N --reason=link --link-of=IS-X
         (cascade-closes when IS-X closes; does NOT cascade-reopen when IS-X reopens.)
    3. rewrite prescriptively: title=intended outcome; context=should/did/direction
    4. apply: dx todo owner triage IS-N --title=... --context=... --type=<ops|impl|ask|tracker> --priority=<1-4> --focus=<ID> --goal=<ID>
       use the active focuses and goals listed above to classify the issue
       type guide:
         ops    = one-time verifiable action (demo/test plan required)
         impl   = durable code change (resolution link required to close)
         ask    = investigation/research/justification (no test plan; may spawn follow-up issues)
         tracker = umbrella issue (closed by its children; solo skips it)
    if the issue is too vague to triage, create clarification questions instead:
      dx question add --target-type=issue --target-id=IS-N --context="<question>" --choices="opt1,opt2,..."
      - prefer --choices when the question has enumerable options; do not embed numbered/lettered lists in freeform --context
      - for multi-stage questions (answer to Q1 changes Q2+), ask only the first stage now; file follow-ups after the answer arrives
    solo will block progress on the issue until all questions are answered.
`

// SimilarIssuesMCPGuidance returns the guidance attached to MCP `issue_add`
// responses when similar issues were found. issueID is the newly created
// issue (e.g. "IS-42").
func SimilarIssuesMCPGuidance(issueID string) string {
	return "SIMILAR ISSUES FOUND — issue created as draft (wip). Review each similar issue before proceeding:\n" +
		"• If a closed issue covers this work: close this as duplicate (issue_close with reason=duplicate, duplicate_of=IS-N) " +
		"OR reopen the original if it was closed prematurely.\n" +
		"• If an open issue overlaps: close this as duplicate, or keep both if genuinely distinct — " +
		"add a blocked_by link if one depends on the other.\n" +
		"• If no real overlap: promote with `dx issue ready " + issueID + "`.\n" +
		"Do NOT ignore similar issues — duplicates waste agent cycles."
}

// SimilarIssuesCLIMessage returns the multi-line CLI output shown after
// `dx issue add` when similar issues were found. The caller is expected to
// have already printed the list of similar issues above this block.
func SimilarIssuesCLIMessage(issueID string) string {
	return fmt.Sprintf(
		"\nIssue created as draft (wip). To promote:\n"+
			"  dx issue ready %s\n"+
			"To close as duplicate:\n"+
			"  dx issue close %s --reason=duplicate --duplicate-of=<IS-N>\n"+
			"To close as a narrow-slice link (cascade-close with target; no reopen-cascade):\n"+
			"  dx issue close %s --reason=link --link-of=<IS-N>\n",
		issueID, issueID, issueID,
	)
}

// MissingTestPlanError returns the error returned when closing a task that
// has neither a stored test plan nor a --test-plan flag.
func MissingTestPlanError(taskID string) error {
	return fmt.Errorf(
		"missing --test-plan: task %s has no stored test plan. Pass --test-plan=\"<how this was verified>\" to close",
		taskID,
	)
}

// MissingTestRefsError returns the error returned when closing an impl task
// that lacks any test / code references linking the verification back to
// code.
func MissingTestRefsError(taskID, issueRef string) error {
	return fmt.Errorf(
		"missing test refs: task %s belongs to impl issue %s. Pass --test-refs=\"<paths or test names>\" or --file <path> so the verification is traceable",
		taskID, issueRef,
	)
}

// ── Solo queue candidate text ──────────────────────────────────────────────
// Each function returns the full agent-actionable text for a solo candidate.
// The text should be self-contained: an agent reading ONLY this text should
// know exactly what to do without consulting any skill file.

// TriageText builds a Hint for an untriaged issue. Covers the full triage
// playbook: read/respond to comments, verify independently, dup-check with
// close-duplicate / close-link syntax, scope-check (3+ areas → tracker),
// prescriptive rewrite, and the clarification-question fallback.
func TriageText(issueID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Triage %s: %s", issueID, title),
		Description: "Untriaged — verify, dup-check, classify, and rewrite.",
		Instructions: fmt.Sprintf(
			"Triage %s: %s.\n\n"+
				"Steps:\n"+
				"1. Read + respond to any unread comments: `dx comment list issue %s` → `dx comment add issue %s --body=...` → "+
				"`dx comment mark-read issue %s --role=llm`.\n"+
				"2. Verify independently — reproduce the bug or read the relevant code/UI before accepting the report.\n"+
				"3. Dup-check — scan `dx issue list` for similar open/closed work.\n"+
				"   - full duplicate: `dx issue close %s --reason=duplicate --duplicate-of=IS-X`\n"+
				"   - narrow slice: `dx issue close %s --reason=link --link-of=IS-X` (cascade-closes with IS-X; no reopen-cascade)\n"+
				"4. Scope check — if the issue touches 3+ distinct areas (schema + CLI + UI + tests), set `--type=tracker` and decompose into child issues instead of working it directly.\n"+
				"5. Rewrite prescriptively — title = intended outcome (not symptom); context covers (a) what should happen, (b) what did happen, (c) implementation direction if known.\n"+
				"6. Apply: `dx todo owner triage %s --title=\"...\" --context=\"...\" --type=<ops|impl|ask|tracker> --priority=<1-4> --focus=<FO-N> --goal=<G-N>`.\n"+
				"   ops = one-time verifiable action; impl = durable code change; ask = investigation; tracker = umbrella (decomposed into children).\n\n"+
				"If the issue is too vague to classify: `dx question add --target-type=issue --target-id=%s --context=\"<what to decide>\" --choices=\"opt1,opt2,...\"` and stop. Solo blocks progress until answered.",
			issueID, title, issueID, issueID, issueID, issueID, issueID, issueID, issueID,
		),
	}
}

// DecomposeIssueText builds a Hint for an issue with no tasks. Covers the full
// task-creation playbook: read issue, split into shippable units, populate
// all structured fields.
func DecomposeIssueText(issueID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Plan %s: %s", issueID, title),
		Description: "No tasks yet — break into implementation steps.",
		Instructions: fmt.Sprintf(
			"Decompose %s: %s.\n\n"+
				"1. Read the issue: `dx issue show %s`. If context is thin, read the referenced code first.\n"+
				"2. Break into tasks — each task is ONE shippable unit (not a multi-day epic). If you find yourself writing >3 tasks, the issue probably should have been a tracker — consider `dx issue edit %s --issue-type=tracker` and decompose into child issues instead.\n"+
				"3. Create each task:\n"+
				"   `dx todo tech add --issue=%s --title=\"<one-line outcome>\" --text=\"<implementation plan, file-by-file>\" --reason=\"<why this task now>\" --test-plan=\"<how it will be verified>\"`\n"+
				"   - `--title` is the outcome-focused headline (shown in the UI, list rows, and solo [dev] messages)\n"+
				"   - `--text` is the step-by-step plan: what to edit, in what files, in what order (markdown supported)\n"+
				"   - `--reason` explains why this is needed at this point in the vertical\n"+
				"   - `--test-plan` is REQUIRED to close via `dev done`. If you skip two flags, keep title + test-plan.\n"+
				"4. Stop after creating tasks — let the solo loop pick the next action.",
			issueID, title, issueID, issueID, issueID,
		),
	}
}

// DevTaskText builds a Hint for a ready development task. Covers reading the
// plan, implementing, and closing with the correct test-plan/test-refs/--file
// grammar (including impl-issue gating).
func DevTaskText(taskID, title, issueRef string) Hint {
	gating := "`."
	if issueRef != "" {
		gating = fmt.Sprintf(
			" --file <path>` (impl issue %s requires test refs — pass `--test-refs=\"<paths|test names>\"` or one+ `--file` flags).",
			issueRef,
		)
	}
	desc := "Dev task ready to implement."
	if issueRef != "" {
		desc = fmt.Sprintf("Dev task on %s — ready to implement.", issueRef)
	}
	return Hint{
		Title:       fmt.Sprintf("Implement %s: %s", taskID, title),
		Description: desc,
		Instructions: fmt.Sprintf(
			"Implement %s: %s.\n\n"+
				"1. Read the plan: `dx todo show %s` — all implementation details are in the `text` field.\n"+
				"2. If the task was created >2 days ago or marked stale, read the referenced files FIRST and verify the work is still needed before editing.\n"+
				"3. Do the work. Commit as you go if helpful.\n"+
				"4. Close: `dx todo dev done %s --test-plan=\"<how it was verified>\"%s\n"+
				"   `--file` grammar: `<path>[:start[-end]][@hash]` — e.g. `--file internal/cli/work/todo.go:1005-1036`. "+
				"Attached to the parent issue as zdx_issue_code_refs.\n"+
				"5. After closing, re-run `dx todo solo --issue=%s` to let the loop pick the next step (more tasks, or issue-closable).",
			taskID, title, taskID, taskID, gating, issueRef,
		),
	}
}

// ClosableIssueText builds a Hint for an issue with all tasks done. Includes
// the close-and-ship sequence and the DX-analysis footer.
func ClosableIssueText(issueID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Close %s: %s", issueID, title),
		Description: "All tasks done — close and ship.",
		Instructions: fmt.Sprintf(
			"All tasks done on %s: %s.\n\n"+
				"1. Verify the work is complete — read `dx issue show %s`, spot-check linked code refs, run targeted tests if in doubt.\n"+
				"2. If anything is missing, add a task instead of leaving the issue open: `dx todo tech add --issue=%s ...`.\n"+
				"3. Close: `dx issue close %s --reason=done`.",
			issueID, title, issueID, issueID, issueID,
		) + CloseAndShipFragment + PostWorkDXAnalysisFragment + StopAfterVerticalFragment,
	}
}

// DecomposeTrackerText builds a Hint for a tracker with no children. Enforces
// the "never work a tracker directly" rule and the per-child dup-check
// discipline.
func DecomposeTrackerText(issueID string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Decompose tracker %s", issueID),
		Description: "Tracker has no child issues — needs decomposition.",
		Instructions: fmt.Sprintf(
			"Tracker %s has no child issues — decompose it.\n\n"+
				"1. Read the tracker context: `dx issue show %s`.\n"+
				"2. Break into concrete child issues, one per vertical (one shippable unit each):\n"+
				"   `dx issue add --title=\"...\" --context=\"...\" --issue-type=impl --parent=%s`\n"+
				"3. After EACH `issue add`, review the similar-issues list in the response:\n"+
				"   - closed similar already covers this: close your new issue as duplicate (or reopen the original if it was closed prematurely)\n"+
				"   - open similar overlaps: close yours as duplicate, or keep both if genuinely distinct and add a blocked_by link\n"+
				"   - no overlap: promote with `dx issue ready <new-IS-N>`\n"+
				"   Skipping this step wastes agent sessions on duplicate child issues.\n"+
				"4. Do NOT implement tracker work inline and do NOT close the tracker manually — it auto-closes when all children close.\n"+
				"5. Stop after decomposition. The loop will pick a child vertical next.",
			issueID, issueID, issueID,
		) + BlockerQuestionCriteriaFragment,
	}
}

// CloseTrackerText builds a Hint for a tracker whose children are all closed.
func CloseTrackerText(issueID string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Close tracker %s", issueID),
		Description: "All child issues closed — tracker ready to close.",
		Instructions: fmt.Sprintf(
			"Tracker %s: all child/blocker issues are closed — tracker is ready to close.\n\n"+
				"Close it: `dx issue close %s --reason=done`. "+
				"Trackers normally auto-close when the last child closes; if this one didn't, closing it manually is fine now that dependencies are clear.",
			issueID, issueID,
		) + PostWorkDXAnalysisFragment + StopAfterVerticalFragment,
	}
}

// UnreadCommentsText builds a Hint for unread comments on an issue.
func UnreadCommentsText(issueID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Comments on %s: %s", issueID, title),
		Description: "Unread LLM comments need a response.",
		Instructions: fmt.Sprintf(
			"Unread comments on %s: %s.\n\n"+
				"1. Read all comments: `dx comment list issue %s`.\n"+
				"2. Understand each — question / clarification request / feedback / decision.\n"+
				"3. Respond if a reply is warranted: `dx comment add issue %s --body=\"<reply>\"`. "+
				"The CLI auto-tags with $DX_AUTHOR_ALIAS (usually `claude`); pass `--as <alias>` only to override.\n"+
				"4. Re-run `dx todo solo --issue=%s` — comments are handled inline, not as a separate vertical.",
			issueID, title, issueID, issueID, issueID,
		),
	}
}

// UnreadFeatureCommentsText builds a Hint for unread comments on a feature.
func UnreadFeatureCommentsText(featureName string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Comments on feature %q", featureName),
		Description: "Unread LLM comments on feature need a response.",
		Instructions: fmt.Sprintf(
			"Unread comments on feature %q.\n\n"+
				"1. Read: `dx comment list feature %s`.\n"+
				"2. Respond to questions/feedback: `dx comment add feature %s --body=\"<reply>\"`.\n"+
				"3. No vertical to run — handle inline and stop.",
			featureName, featureName, featureName,
		),
	}
}

// StaleCommentsText builds a Hint for N aged unread comments on a target
// (issue / task / feature). The text tells the agent how to read, respond to,
// and mark the comments as read.
func StaleCommentsText(count int, targetType, targetID string, lastCommentID int32) Hint {
	return Hint{
		Title:       fmt.Sprintf("%d stale comment(s): %s %s", count, targetType, targetID),
		Description: "Comments aged >24h without response.",
		Instructions: fmt.Sprintf(
			"%d aged unread comment(s) on %s %s (latest C-%d).\n\n"+
				"1. Read all unread: `dx comment list %s %s`.\n"+
				"2. Respond if a reply is warranted: `dx comment add %s %s --body=\"<reply>\"`.\n"+
				"These are older than 24h — prioritize answering before new work.",
			count, targetType, targetID, lastCommentID,
			targetType, targetID,
			targetType, targetID,
		),
	}
}

// OrphanTaskText builds a Hint for a task with no parent issue.
func OrphanTaskText(taskID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Orphan task %s: %s", taskID, title),
		Description: "Task has no parent issue.",
		Instructions: fmt.Sprintf(
			"Orphan task %s (no parent issue): %s.\n\n"+
				"1. Read: `dx todo show %s`. Decide: done, superseded, or still needed?\n"+
				"2. If done/superseded: `dx todo dev done %s --test-plan=\"<how verified or superseded by <ref>\">`.\n"+
				"3. If still needed but no host issue exists: file one — `dx issue add --title=\"...\" --context=\"...\"` — then adopt the task: `dx task adopt %s --issue=<IS-N>`.\n"+
				"4. If still needed and a host issue already exists: `dx task adopt %s --issue=<IS-N>` directly.\n"+
				"Stop after handling — one orphan per session.",
			taskID, title, taskID, taskID, taskID, taskID,
		),
	}
}

// ReviewPendingTaskText builds a Hint for a done task awaiting review verdict.
// The agent should run dx todo dev review with --verdict to approve or reject.
func ReviewPendingTaskText(taskID, title, issueRef string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Review %s: %s", taskID, title),
		Description: fmt.Sprintf("Task done on %s — needs review verdict before issue can close.", issueRef),
		Instructions: fmt.Sprintf(
			"Task %s (%s) is done and waiting for a review verdict.\n\n"+
				"1. Read the task and its work: `dx todo dev review %s` (prints details, code refs, test plan).\n"+
				"2. Evaluate whether the implementation is correct and complete.\n"+
				"3. Submit your verdict:\n"+
				"   approve: `dx todo dev review %s --verdict=approve [--body=\"<note>\"]`\n"+
				"   reject:  `dx todo dev review %s --verdict=reject --body=\"<what needs to change>\"`\n"+
				"4. After approving, re-run `dx todo solo --issue=%s` to let the loop close or pick next steps.\n"+
				"   After rejecting, a new dev task will be needed — create one: `dx todo tech add --issue=%s --title=\"...\" --text=\"...\"`.",
			taskID, title, taskID, taskID, taskID, issueRef, issueRef,
		),
	}
}

// ReviewStaleTaskText builds a Hint for a stale task (created but never claimed,
// enough time passed that the code may have moved).
func ReviewStaleTaskText(taskID, title string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Stale task %s: %s", taskID, title),
		Description: "Created but never claimed — verify relevance before implementing.",
		Instructions: fmt.Sprintf(
			"Review stale task %s: %s.\n\n"+
				"This task was created but never claimed, and enough time has passed that the codebase may have changed.\n\n"+
				"1. Read the plan: `dx todo show %s`. Identify the files it references.\n"+
				"2. Read those files NOW — verify the described work is still needed. The state may have moved under it.\n"+
				"3. If already implemented or superseded: `dx todo dev done %s --test-plan=\"superseded by <ref>\"`.\n"+
				"4. If still needed: proceed as a normal dev task — implement, then close with `--test-plan` and `--file`/`--test-refs`.\n"+
				"Do NOT start editing code until you have verified the task is still relevant.",
			taskID, title, taskID, taskID,
		),
	}
}

// BootstrapText builds a Hint for a brand-new project with no issues/features.
func BootstrapText(slug string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Bootstrap %q", slug),
		Description: "New project — no issues or features yet.",
		Instructions: fmt.Sprintf(
			"Project %q is new — no issues or features yet.\n\n"+
				"1. Classify + scaffold: `dx doctor --fix`. This picks a project classification (library/tool/service/saas/site) and seeds the maturity vine.\n"+
				"2. Scan the codebase thoroughly: entry points (main packages, server routes, CLI commands), data model (migrations, schema, ORM models), external integrations (APIs, queues, auth), UI (pages, components), build/deploy (Dockerfile, CI, Makefile).\n"+
				"3. Create a feature for each conceptual capability: `dx feature add <name> --desc=\"<what it does>\"`. "+
				"Set `--kind=direct` (deposits goal currency) or `--kind=multiplier` (amplifies other features; needs metric + baseline + target + graph_url). "+
				"Link to a goal: `dx feature set <name> --goal <G-N>`.\n"+
				"4. File a setup issue: `dx issue add --title=\"Integrate project with zdx tooling\" --context=\"Set up .zdx/config.yaml close-hooks (lint, gen), configure components, and verify dx todo solo cycle works end-to-end.\"`\n"+
				"5. Re-run `dx todo solo` — the triage flow will engage on the setup issue.",
			slug,
		) + BlockerQuestionCriteriaFragment,
	}
}

// DemoGapText builds a Hint for a spec that has no demo test linked.
func DemoGapText(specID int32, description, featureName string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Demo gap: spec %d on feature %q", specID, featureName),
		Description: description,
		Instructions: fmt.Sprintf(
			"Spec %d (%s) on feature %q has no demo.\n\n"+
				"1. Check if a demo test already exists that could cover this spec: `dx test list --layer=demo`.\n"+
				"2. Link an existing demo: `dx spec link %d <test-id>`.\n"+
				"3. Or write a new demo test, then link it.\n"+
				"Demos are verifiable walk-throughs of the spec's behavior — prefer small, focused scenarios.\n"+
				"Stop after linking/filing — the demo test itself is a separate dev task.",
			specID, description, featureName, specID,
		),
	}
}

// NoGoalsText builds a Hint for a project with no goals.
func NoGoalsText() Hint {
	return Hint{
		Title:       "No goals defined",
		Description: "Goals anchor features to measurable outcomes.",
		Instructions: "Project has no goals defined.\n\n" +
			"Goals anchor features to measurable outcomes.\n" +
			"1. Talk with the owner (or read plan/product docs) to list top-level objectives.\n" +
			"2. For each: `dx goal add \"<goal title>\"`.\n" +
			"3. Optionally quantify: `dx goal set <G-N> --metric-name=<name> --metric-unit=<unit>`.\n" +
			"Stop after adding — let the next solo pick guide what's next.",
	}
}

// NoConstraintsText builds a Hint for a project with no constraints.
func NoConstraintsText() Hint {
	return Hint{
		Title:       "No constraints defined",
		Description: "Constraints capture non-negotiable boundaries.",
		Instructions: "Project has no constraints defined.\n\n" +
			"Constraints capture non-negotiable boundaries (latency budgets, compliance requirements, data locality, etc.).\n" +
			"For each: `dx constraint add \"<constraint>\"`.\n" +
			"Stop after adding — let the next solo pick drive next steps." +
			BlockerQuestionCriteriaFragment,
	}
}

// StandupOverdueText builds a Hint for an overdue standup check-in.
//
// The standup report is the agent's value proposition to the stakeholder:
// read from a beach, learn whether the project is healthy, what is on fire,
// and what the plan is. A receipt ("check-in acknowledged") is not a report.
// The builder is persona-agnostic so additional roles (e.g. a future Ops
// persona) can be added without rewriting.
func StandupOverdueText(role string) Hint {
	roleCap := capitalize(role)
	var structure string
	switch role {
	case "owner":
		structure = "Owner report sections (spread across --assessment / --concerns / --next):\n" +
			"   1. Progress summary — what shipped since last standup, grouped by feature/focus, one-line descriptions.\n" +
			"   2. Current state — open issues by priority, WIP count, blocked items and WHY they're blocked.\n" +
			"   3. Feature health — progressing vs. stalled, spec coverage gaps, deferred specs and their blockers.\n" +
			"   4. Focus alignment — active focuses and whether work is tracking toward their goals.\n" +
			"   5. Risks & attention — items needing human decision, each with recommended action or explicit \"watching this because…\".\n" +
			"   6. Near-term plan — next cycle based on the queue.\n" +
			"   7. Long-term trajectory — goal progress, maturity-gradient gaps, strategic risks.\n"
	case "tech":
		structure = "Tech report sections (spread across --assessment / --concerns / --next):\n" +
			"   1. What changed — schema migrations, API changes, new endpoints, dependency updates since last standup.\n" +
			"   2. System health — error rates, timing KPIs, counter trends (observability pages).\n" +
			"   3. Architecture notes — refactors, new patterns, technical debt added or paid down.\n" +
			"   4. Test coverage — results, new tests, failing tests, deferred specs.\n" +
			"   5. Identified risks — performance regressions, security concerns, scaling issues, fragile areas.\n" +
			"   6. Improvement areas — DX friction from post-work analysis, recurring churn patterns, tooling gaps.\n"
	default:
		structure = capitalize(role) + " report — cover: what changed, current state, risks, and near-term plan. " +
			"Be quantitative. Spread content across --assessment / --concerns / --next.\n"
	}

	techExtras := ""
	if role == "tech" {
		techExtras = "   - observability: timings / errors / counters pages — system KPIs\n"
	}

	return Hint{
		Title:       roleCap + " standup overdue",
		Description: roleCap + " standup check-in overdue.",
		Instructions: fmt.Sprintf(
			"%s standup check-in is overdue.\n\n"+
				"A standup is a stakeholder-facing analysis, not a receipt. The stakeholder should read it from a beach and know: project healthy / no blockers, OR exactly what needs their attention and what's being done about it. Never \"acknowledge\" or \"summarize back\" the previous entry — they wrote it.\n\n"+
				"1. Gather data BEFORE writing (an agent with no data produces vague assessments):\n"+
				"   - `dx issue list` — open / closed / WIP by priority, status, date\n"+
				"   - `dx feature list` / `dx feature show <name>` — feature health, spec coverage, stale features\n"+
				"   - `dx focus list` / `dx focus status` — active focuses and their feature linkage\n"+
				"   - `dx goal list` — goals and metric presence\n"+
				"   - `dx todo list` — queue state, blocked items\n"+
				"   - `dx standup show --role=%s` — previous entry; compute deltas (do NOT regurgitate it)\n"+
				"   - `git log --since=<prev-entry-date>` — what actually shipped (commits, not claims)\n"+
				"%s"+
				"2. %s"+
				"3. Submit via `dx standup checkin --role=%s --tldr=\"...\" --assessment=\"...\" --concerns=\"...\" --next=\"...\" --project-root=$(git rev-parse --show-toplevel)`.\n"+
				"   - `--tldr` MUST lead with the stakeholder-actionable bottom line: either `project healthy, no blockers` or `N items need your attention: …`. Never bury the verdict.\n"+
				"   - `--assessment` carries the quantitative state (section 1–2 / 1–3 above).\n"+
				"   - `--concerns` carries risks, stalled work, human-decision items (section 3–5).\n"+
				"   - `--next` carries the near-term and long-term plan (section 6–7 / sections 4–6 for tech).\n\n"+
				"Anti-patterns (these will be rejected in review):\n"+
				"- Do NOT just acknowledge the previous entry — that is not a report.\n"+
				"- Do NOT summarize the previous journal entry back to the stakeholder — they wrote it.\n"+
				"- Do NOT produce vague qualitative assessments (\"strong velocity\") without backing numbers.\n"+
				"- Do NOT skip the report because \"nothing to do\" — an empty queue is itself a reportable state; say so explicitly.\n"+
				"- Do NOT bury problems in positive framing. If something is wrong, say it plainly.\n"+
				"- Do NOT close this item with `dx standup review` — review is for acknowledging someone ELSE's entry, not for writing your own. Use `dx standup checkin`.\n\n"+
				"Stop after submitting — one standup per session.",
			roleCap, role, techExtras, structure, role,
		),
	}
}

// JournalReviewText builds a Hint for reviewing a generated journal check-in.
func JournalReviewText(role string) Hint {
	return Hint{
		Title:       capitalize(role) + " journal review",
		Description: "Generated check-in needs review.",
		Instructions: fmt.Sprintf(
			"Review generated %s check-in.\n\n"+
				"1. Read the latest entry: `dx journal show`.\n"+
				"2. Verify the data is accurate and the assessment is fair.\n"+
				"3. If corrections are needed, submit an updated check-in: `dx standup checkin --role=%s --project-root=$(git rev-parse --show-toplevel)`.\n"+
				"Stop after review — one journal entry per session.",
			role, role,
		),
	}
}

// NoSpecsText builds a Hint for a feature with no specs.
func NoSpecsText(featureName string) Hint {
	return Hint{
		Title:       fmt.Sprintf("No specs: %q", featureName),
		Description: "Feature has no specs — add concerns before verification.",
		Instructions: fmt.Sprintf(
			"Feature %q has no specs.\n\n"+
				"Specs define concerns on a feature (functional, latency, security, ux, compatibility) at kind=must/should/nice-to-have.\n"+
				"1. Read the feature: `dx feature show %s`.\n"+
				"2. For each concern on that feature, add a spec:\n"+
				"   `dx spec add %s --kind=must --concern-type=functional --desc=\"<what this must do / avoid>\"`\n"+
				"Stop after adding specs — cross-cutting checks (test coverage, demos) will surface next.",
			featureName, featureName, featureName,
		) + BlockerQuestionCriteriaFragment,
	}
}

// StaleFeatureText builds a Hint for a feature not reviewed in >30 days.
func StaleFeatureText(featureName string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Stale feature: %q", featureName),
		Description: "Feature not reviewed in >30 days.",
		Instructions: fmt.Sprintf(
			"Feature %q not reviewed in >30 days.\n\n"+
				"1. Review: `dx feature show %s` — read specs, test coverage, linked issues, recent commits touching the feature area.\n"+
				"2. Update the description or specs if reality has drifted: `dx feature set %s --desc=\"...\"` / `dx spec add ...`.\n"+
				"3. Mark reviewed: `dx feature review %s`.\n"+
				"Stop after review — one feature per session.",
			featureName, featureName, featureName, featureName,
		),
	}
}

// NoTestRefsText builds a Hint for a spec with no test coverage.
func NoTestRefsText(specID int32, description, featureName string) Hint {
	return Hint{
		Title:       fmt.Sprintf("No test refs: spec %d on %q", specID, featureName),
		Description: description,
		Instructions: fmt.Sprintf(
			"These are ./bin/dx shell commands — run via Bash, NOT via any MCP tool.\n\n"+
				"Spec %d (%s) on feature %q has no test refs.\n\n"+
				"Discover test IDs in the DB:\n"+
				"   `./bin/dx test list --from-db | grep <keyword>`\n"+
				"Option A — if a test already covers this spec:\n"+
				"   `./bin/dx spec link %d <test-id>`\n"+
				"Option B — if no test exists, file a task for it:\n"+
				"   `./bin/dx todo tech add --spec=%d --title=\"Test spec %d: %s\" --text=\"<test approach>\" --test-plan=\"<how the test verifies the spec>\"`\n"+
				"The --spec flag links the task to this spec and suppresses this nudge while the task is open.\n"+
				"Stop after linking/filing — the test itself is a separate dev task.",
			specID, description, featureName, specID, specID, specID, description,
		),
	}
}

// ── Maturity-kind texts ────────────────────────────────────────────────────
// Surfaced by handlers_solo.go when ListOpenMaturityItems returns items.
// These are maturity-gradient items — the project is healthy enough to work
// but could be healthier. Each text explains the kind and when to resolve
// directly vs. file a blocker question.

// AttributeFeatureText — a feature exists that's not linked to a goal or
// parent feature.
func AttributeFeatureText(title, description string) Hint {
	return Hint{
		Title:       title,
		Description: description,
		Instructions: "A feature is not attributed to a goal or parent. Unattributed features are hard to prioritize.\n\n" +
			"1. Identify the feature: `dx feature list` (look for features with no goal / no parent).\n" +
			"2. If the right goal is obvious from the feature name/desc: `dx feature set <name> --goal <G-N>`.\n" +
			"3. If a parent feature is obvious (this feature is a sub-capability of a larger one): `dx feature set <name> --parent <parent-feature-name>`.\n" +
			"4. If the answer requires product judgment (which goal? decomposition not obvious?): " +
			"`dx question add --target-type=feature --target-id=<feature-name> --context=\"<what to decide>\" --choices=\"G-1,G-2,parent-X,new-goal,...\"` and stop.\n" +
			"Stop after applying or filing — one nudge per session.",
	}
}

// QuantifyGoalText — a goal has no metric_name / metric_unit.
func QuantifyGoalText(title, description string) Hint {
	return Hint{
		Title:       title,
		Description: description,
		Instructions: "A goal has no metric. Unmeasured goals cannot be tracked.\n\n" +
			"1. Identify: `dx goal list` (look for goals without metric_name/metric_unit).\n" +
			"2. If the metric is obvious from the goal's nature: `dx goal set <G-N> --metric-name=<name> --metric-unit=<unit>` (e.g. `--metric-name=weekly-active-users --metric-unit=users`).\n" +
			"3. If the metric requires product judgment (which signal measures success?): " +
			"`dx question add --target-type=goal --target-id=G-N --context=\"<what signal should quantify this goal>\" --choices=\"opt1,opt2,...\"` and stop.\n" +
			"Stop after applying or filing.",
	}
}

// InstrumentFeatureText — a multiplier feature is missing baseline, target,
// or graph_url.
func InstrumentFeatureText(title, description string) Hint {
	return Hint{
		Title:       title,
		Description: description,
		Instructions: "A multiplier feature is missing baseline, target, or graph_url. Multipliers amplify other features — without instrumentation they cannot be validated.\n\n" +
			"1. Identify: `dx feature list --kind=multiplier` (look for fields left unset).\n" +
			"2. If a baseline and target can be derived from existing data or product intent: `dx feature set <name> --baseline=<n> --target=<n> --graph-url=<url>`.\n" +
			"3. If instrumentation requires code changes (wire metrics, add observability): file an impl issue — `dx issue add --title=\"Instrument multiplier feature <name>\" --context=\"<what to measure, where>\"` — and stop.\n" +
			"4. If baseline/target requires product judgment: `dx question add --target-type=feature --target-id=<name> --context=\"<what to set>\" --choices=\"...\"` and stop.\n" +
			"Stop after the action — one nudge per session.",
	}
}

// DecomposeFeatureText — a feature has too many specs (>8) and should be
// split into child features.
func DecomposeFeatureText(title, description string) Hint {
	return Hint{
		Title:       title,
		Description: description,
		Instructions: "A feature has >8 specs — too broad. Split into focused child features.\n\n" +
			"1. Identify: `dx feature show <name>` — read the specs to find natural groupings.\n" +
			"2. Create child features: `dx feature add <child-name> --desc=\"...\" --parent=<name>` for each grouping.\n" +
			"3. Re-assign specs from parent to child as needed (specs aren't auto-moved).\n" +
			"4. If grouping requires product judgment: `dx question add --target-type=feature --target-id=<name> --context=\"<proposed split>\" --choices=\"opt1,opt2,...\"` and stop.\n" +
			"Stop after decomposing or filing.",
	}
}

// RespondDiscussionText builds a Hint for a discussion whose last message is
// from the user. The agent should read the full history and post a direct reply.
func RespondDiscussionText(discussionID, title, lastMessage string) Hint {
	return Hint{
		Title:       fmt.Sprintf("Reply in %s: %s", discussionID, title),
		Description: lastMessage,
		Instructions: fmt.Sprintf(
			"User message awaiting reply in discussion %s: %s.\n\n"+
				"Last user message:\n> %s\n\n"+
				"1. Read the full history: `dx discussion show %s`.\n"+
				"2. Compose a response that addresses the user's message directly.\n"+
				"3. Reply: `dx discussion reply %s --message=\"<your response>\"`.\n"+
				"   Use `dx discussion reply` (agent post, no LLM call) — the agent IS the respondent here.\n"+
				"Stop after replying — one discussion per session.",
			discussionID, title, lastMessage, discussionID, discussionID,
		),
	}
}

// ReviewDeferredSpecText builds a Hint for a spec whose deferral blockers are all closed.
// The agent should evaluate whether to write the test, mark the spec resolved, or re-defer.
func ReviewDeferredSpecText(specID int32, description string) Hint {
	id := fmt.Sprintf("%d", specID)
	return Hint{
		Title:       fmt.Sprintf("Review deferred spec %d: %s", specID, description),
		Description: "All blocking issues closed — re-evaluate this deferred spec.",
		Instructions: fmt.Sprintf(
			"Deferred spec %s (%s) has no open blockers — time to re-evaluate.\n\n"+
				"1. Read the spec and its deferrals: `dx spec show %s`.\n"+
				"2. Choose one:\n"+
				"   a. If the concern is now testable: write or link the test — `dx spec link %s <test-id>` — then mark the deferral resolved: `dx spec defer remove %s --issue=<IS-N>`.\n"+
				"   b. If the concern is not yet addressable: re-defer with a new blocker — `dx spec defer %s --blocked-by=<new-IS-N> --note=\"<why>\"`.\n"+
				"   c. If the spec is no longer relevant: remove it — `dx spec delete %s`.\n"+
				"3. Stop after handling — one spec per session.",
			id, description, id, id, id, id, id,
		),
	}
}

// MaturityDefaultText is the fallback Hint when a maturity item's kind has no
// dedicated builder.
func MaturityDefaultText(title, description string) Hint {
	return Hint{
		Title:       title,
		Description: description,
		Instructions: "Address this maturity item. Read the target and resolve directly if the fix is obvious, otherwise file a blocker question targeting the affected entity (`dx question add --target-type=... --target-id=... --context=\"<what to decide>\"`).\n" +
			"Stop after the action — one nudge per session.",
	}
}

// MaturityTextForKind dispatches on the maturity item's kind, returning the
// kind-specific Hint or the default when no builder exists.
func MaturityTextForKind(kind, title, description string) Hint {
	switch kind {
	case "owner:attribute-feature":
		return AttributeFeatureText(title, description)
	case "owner:quantify-goal":
		return QuantifyGoalText(title, description)
	case "tech:instrument-feature":
		return InstrumentFeatureText(title, description)
	case "owner:decompose-feature":
		return DecomposeFeatureText(title, description)
	}
	return MaturityDefaultText(title, description)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
