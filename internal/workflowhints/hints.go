// Package workflowhints centralizes the workflow-guidance strings surfaced to
// LLM agents and CLI users. Keeping hints in one place means that changing a
// workflow rule (e.g. how to handle similar issues, or what counts as a valid
// test plan) updates every surface — solo queue candidates, MCP tool
// responses, and CLI output — in lock-step.
package workflowhints

import "fmt"

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

// StaleCommentsText builds the solo-candidate text for N aged unread comments
// on a target (issue / task / feature). The text tells the agent how to read,
// respond to, and mark the comments as read.
func StaleCommentsText(count int, targetType, targetID string, lastCommentID int32) string {
	return fmt.Sprintf(
		"%d unread comment(s) on %s %s (latest C-%d). "+
			"Read all unread comments via `dx comment list %s %s`. "+
			"Respond if needed via `dx comment add %s %s --body=...`. "+
			"Then mark all read: `dx comment mark-read %s %s --role=llm`.",
		count, targetType, targetID, lastCommentID,
		targetType, targetID,
		targetType, targetID,
		targetType, targetID,
	)
}

// DecomposeTrackerText builds the solo-candidate text for a tracker issue
// that has no child issues yet.
func DecomposeTrackerText(issueID string) string {
	return fmt.Sprintf(
		"Tracker %s has no child issues — decompose it. "+
			"Read the tracker context, then create child issues: "+
			"`dx issue add --title=\"...\" --context=\"...\" --issue-type=impl --parent=%s` "+
			"for each shippable unit of work.",
		issueID, issueID,
	)
}

// CloseTrackerText builds the solo-candidate text for a tracker whose
// dependencies are all closed.
func CloseTrackerText(issueID string) string {
	return fmt.Sprintf("Tracker %s: all dependencies closed — ready to close", issueID)
}

// DemoGapText builds the solo-candidate text for a spec that has no demo
// test linked.
func DemoGapText(specID int32, description, featureName string) string {
	return fmt.Sprintf(
		"Spec %d (%s) on %q has no demo — write a new demo test or link an existing one (dx spec link %d <test-id>)",
		specID, description, featureName, specID,
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

// UnreadCommentsText builds text for unread comments on an issue.
func UnreadCommentsText(issueID, title string) string {
	return fmt.Sprintf(
		"Unread comments on %s: %s. "+
			"Read all comments via `dx comment list issue %s`. "+
			"Respond to questions/feedback via `dx comment add issue %s --body=\"...\"`. "+
			"Then mark all read: `dx comment mark-read issue %s --role=llm`.",
		issueID, title, issueID, issueID, issueID,
	)
}

// UnreadFeatureCommentsText builds text for unread comments on a feature.
func UnreadFeatureCommentsText(featureName string) string {
	return fmt.Sprintf(
		"Unread comments on feature %q. "+
			"Read all comments via `dx comment list feature %s`. "+
			"Respond to questions/feedback via `dx comment add feature %s --body=\"...\"`. "+
			"Then mark all read: `dx comment mark-read feature %s --role=llm`.",
		featureName, featureName, featureName, featureName,
	)
}

// NoGoalsText builds text for a project with no goals.
func NoGoalsText() string {
	return "Project has no goals defined. " +
		"Goals anchor features to measurable outcomes. " +
		"Run `dx goal add \"<goal title>\"` for each top-level objective. " +
		"Optionally add metrics: `dx goal set <G-N> --metric-name=<name> --metric-unit=<unit>`."
}

// NoConstraintsText builds text for a project with no constraints.
func NoConstraintsText() string {
	return "Project has no constraints defined. " +
		"Constraints capture non-negotiable boundaries (latency budgets, compliance requirements, etc.). " +
		"Run `dx constraint add \"<constraint>\"` for each."
}

// StandupOverdueText builds text for an overdue standup check-in.
func StandupOverdueText(role string) string {
	return fmt.Sprintf(
		"%s standup check-in overdue. "+
			"Gather project data and produce a stakeholder-facing report. "+
			"Data sources: `dx issue list`, `dx feature list`, `dx goal list`, `dx focus list`, recent `git log`. "+
			"Submit via `dx journal checkin --%s`. "+
			"The report should cover: progress since last standup, current state (open/blocked items), "+
			"risks & attention areas, and near-term plan. Quantitative where possible.",
		capitalize(role), role,
	)
}

// JournalReviewText builds text for reviewing a generated journal check-in.
func JournalReviewText(role string) string {
	return fmt.Sprintf(
		"Review generated %s check-in. "+
			"Read the latest journal entry via `dx journal show`. "+
			"Verify the data is accurate and the assessment is fair. "+
			"If corrections are needed, submit an updated check-in via `dx journal checkin --%s`.",
		role, role,
	)
}

// TriageText builds text for an untriaged issue.
func TriageText(issueID, title string) string {
	return fmt.Sprintf(
		"Triage %s: %s. "+
			"Steps: (1) Read comments via `dx comment list issue %s` — respond to any unread. "+
			"(2) Verify independently — reproduce or read the relevant code. "+
			"(3) Dup-check — `dx issue list` for similar open/closed issues. "+
			"(4) Scope check — if 3+ distinct areas, set type=tracker not impl. "+
			"(5) Rewrite prescriptively — title=intended outcome, context=should/did/direction. "+
			"(6) Apply: `dx todo owner triage %s --title=\"...\" --context=\"...\" --type=<ops|impl|ask|tracker> --priority=<1-4> --focus=<FO-N> --goal=<G-N>`.",
		issueID, title, issueID, issueID,
	)
}

// NoSpecsText builds text for a feature with no specs.
func NoSpecsText(featureName string) string {
	return fmt.Sprintf(
		"Feature %q has no specs. "+
			"Specs define concerns (functional, latency, security, ux, compatibility) on a feature. "+
			"Run `dx feature show %s` to understand the feature, then add specs: "+
			"`dx spec add %s --kind=must --concern-type=functional --desc=\"...\"` for each concern.",
		featureName, featureName, featureName,
	)
}

// StaleFeatureText builds text for a feature not reviewed in >30 days.
func StaleFeatureText(featureName string) string {
	return fmt.Sprintf(
		"Feature %q not reviewed in >30 days. "+
			"Run `dx feature show %s` to review current state: specs, test coverage, linked issues. "+
			"Update if needed via `dx feature set %s --desc=\"...\"`. "+
			"Mark reviewed: `dx feature review %s`.",
		featureName, featureName, featureName, featureName,
	)
}

// NoTestRefsText builds text for a spec with no test coverage.
func NoTestRefsText(specID int32, description, featureName string) string {
	return fmt.Sprintf(
		"Spec %d (%s) on feature %q has no test refs. "+
			"Either write a test that covers this spec and link it: `dx spec link %d <test-id>`, "+
			"or file a task to add coverage: `dx todo tech add --title=\"Test spec %d\" --text=\"...\" --test-plan=\"...\"`.",
		specID, description, featureName, specID, specID,
	)
}

// ClosableIssueText builds text for an issue with all tasks done.
func ClosableIssueText(issueID, title string) string {
	return fmt.Sprintf(
		"All tasks done on %s: %s. "+
			"Verify the work is complete, then close: `dx issue close %s --reason=done`. "+
			"If anything is missing, add a new task instead of leaving the issue open.",
		issueID, title, issueID,
	)
}

// DecomposeIssueText builds text for an issue with no tasks.
func DecomposeIssueText(issueID, title string) string {
	return fmt.Sprintf(
		"Decompose %s: %s. "+
			"Read the issue context, then create tasks: "+
			"`dx todo tech add --issue=%s --title=\"<outcome>\" --text=\"<implementation plan>\" --reason=\"<why>\" --test-plan=\"<verification>\"`. "+
			"Each task should be a single shippable unit. Populate all structured fields.",
		issueID, title, issueID,
	)
}

// DevTaskText builds text for a ready development task.
func DevTaskText(taskID, title, issueRef string) string {
	text := fmt.Sprintf(
		"Implement %s: %s. "+
			"Read the task's implementation plan via `dx todo show %s`. "+
			"Do the work, then close: `dx todo dev done %s --test-plan=\"<how it was verified>\"",
		taskID, title, taskID, taskID,
	)
	if issueRef != "" {
		text += fmt.Sprintf(" --file <test-file-path>` (impl issue %s requires test refs).", issueRef)
	} else {
		text += "`."
	}
	return text
}

// OrphanTaskText builds text for a task with no parent issue.
func OrphanTaskText(taskID, title string) string {
	return fmt.Sprintf(
		"Orphan task %s (no parent issue): %s. "+
			"Check if the work is already done or stale — if so, close: `dx todo dev done %s`. "+
			"If still needed, file an issue to host it: `dx issue add --title=\"...\"`, then link.",
		taskID, title, taskID,
	)
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}
