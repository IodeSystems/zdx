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
