package e2e

import (
	"fmt"
	"net/http"
	"testing"
)

// TestReadCommentsResolveAdvancesSeenMark is the IS-1040 regression: resolving
// a read:comments synthetic todo must mark the target's comments as seen so the
// regenerator stops emitting the candidate every iteration. Before the fix,
// ListTargetsWithComments returned every issue with any comment, the agent
// could never clear the synthetic todo, the post-resolve cycle check fired,
// and UpsertTodo's ON CONFLICT clause then clobbered the block flag — so the
// loop spun on the same key indefinitely.
func TestReadCommentsResolveAdvancesSeenMark(t *testing.T) {
	d := NewApiDriver(t, "rc-cycle", "ReadComments Cycle")
	sc := Given(d).TriagedIssue("Commented issue", "test", 3).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	// Seed one comment so the synthetic read:comments candidate appears.
	d.AddComment("issue", targetID, "Initial question")

	// Claim the read:comments todo. Skip past any non-matching kinds the
	// queue might emit first (priority order varies with seed data).
	var claimed TodoItem
	for i := 0; i < 10; i++ {
		c, status := agentClaimNext(t, d.Slug, fmt.Sprintf("rc-agent-%d", i))
		if status != http.StatusOK {
			t.Fatalf("agent/claim: status=%d on attempt %d", status, i)
		}
		if c.Kind == "read:comments" && c.TargetID == targetID {
			claimed = c
			break
		}
	}
	if claimed.ID == 0 {
		t.Fatal("never claimed the read:comments todo for the seeded issue")
	}

	// Resolve the todo. Without the IS-1040 fix the post-resolve cycle check
	// regenerates the queue, sees the same key (because the regenerator emits
	// for any comment that exists, regardless of read state), and returns
	// cycle_detected=true. With the fix the seen high-water mark advances
	// before the cycle check runs, so the candidate is gone and no cycle.
	var rel struct {
		OK            bool `json:"ok"`
		CycleDetected bool `json:"cycle_detected"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/agent/release",
		map[string]any{"id": claimed.ID, "agent_id": "rc-resolver", "resolve": true}, &rel))

	if rel.CycleDetected {
		t.Fatal("cycle_detected=true: read:comments todo regenerated after resolve — seen mark not advancing")
	}

	// Re-evaluate the queue: the read:comments candidate must NOT reappear
	// for this target until a newer comment is added.
	items := d.EvaluateQueue("")
	for _, it := range items {
		if it.Kind == "read:comments" && it.TargetID == targetID {
			t.Errorf("read:comments candidate %q still present after resolve — seen mark did not suppress regen", it.Key)
		}
	}

	// New comment after the seen mark advances should re-surface the todo
	// (proves the mark advances rather than permanently disables the source).
	d.AddComment("issue", targetID, "Follow-up question after resolve")
	items = d.EvaluateQueue("")
	found := false
	for _, it := range items {
		if it.Kind == "read:comments" && it.TargetID == targetID {
			found = true
			break
		}
	}
	if !found {
		t.Error("new comment did not re-surface read:comments candidate — watermark may be set too high")
	}
}

// TestReadCommentsClearsAfterAgentReply is the IS-687 regression: after an
// agent replies to a comment (author_alias != ”), the read:comments candidate
// must NOT re-surface — the agent has already done its job and should not be
// asked to read its own reply. Only a new human comment (author_alias = ”)
// should re-surface the candidate.
func TestReadCommentsClearsAfterAgentReply(t *testing.T) {
	d := NewApiDriver(t, "rc-agent-reply", "ReadComments Agent Reply")
	sc := Given(d).TriagedIssue("Agent reply issue", "test", 3).Build()
	issueID := sc.Issues[0]
	targetID := fmt.Sprintf("IS-%d", issueID)

	// Seed one user comment so the synthetic read:comments candidate appears.
	d.AddComment("issue", targetID, "User question")

	// Claim and resolve the read:comments todo.
	var claimed TodoItem
	for i := 0; i < 10; i++ {
		c, status := agentClaimNext(t, d.Slug, fmt.Sprintf("rc-agent-reply-%d", i))
		if status != http.StatusOK {
			t.Fatalf("agent/claim: status=%d on attempt %d", status, i)
		}
		if c.Kind == "read:comments" && c.TargetID == targetID {
			claimed = c
			break
		}
	}
	if claimed.ID == 0 {
		t.Fatal("never claimed the read:comments todo for the seeded issue")
	}

	var rel struct {
		OK            bool `json:"ok"`
		CycleDetected bool `json:"cycle_detected"`
	}
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/agent/release",
		map[string]any{"id": claimed.ID, "agent_id": "rc-agent-reply", "resolve": true}, &rel))

	// Agent posts a reply (author_alias != '').
	// This simulates the agent responding to the user comment.
	// Without the TK-1455 fix, this new comment (with a higher ID) would
	// re-surface the read:comments candidate because it's newer than the
	// seen watermark, creating a cycle.
	d.AddCommentAs("issue", targetID, "Agent reply to the user", "agent-alias")

	// Re-evaluate: read:comments must NOT appear because the latest comment
	// is agent-authored.
	items := d.EvaluateQueue("")
	for _, it := range items {
		if it.Kind == "read:comments" && it.TargetID == targetID {
			t.Errorf("read:comments candidate %q present after agent reply — should be suppressed", it.Key)
		}
	}

	// A new user comment should re-surface the candidate.
	d.AddComment("issue", targetID, "Another user question")
	items = d.EvaluateQueue("")
	found := false
	for _, it := range items {
		if it.Kind == "read:comments" && it.TargetID == targetID {
			found = true
			break
		}
	}
	if !found {
		t.Error("new user comment did not re-surface read:comments candidate")
	}
}
