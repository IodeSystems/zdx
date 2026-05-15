package agent

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// claimedTodo is the wire shape returned by POST /api/dx/agent/claim (or
// /claim-any for global agents) and used by every code path that walks the
// claim/renew/release lifecycle. Lives in its own file rather than next to
// one provider so all providers can adopt atomic claims without an import
// cycle through claude.go.
type claimedTodo struct {
	ID              int32  `json:"id"`
	Text            string `json:"text"`
	Key             string `json:"key"`
	Kind            string `json:"kind"`
	TargetType      string `json:"target_type"`
	TargetID        string `json:"target_id"`
	IssueRef        string `json:"issue_ref"`
	TargetBranch    string `json:"target_branch,omitempty"`
	Priority        int32  `json:"priority"`
	ClaimedBy       string `json:"claimed_by"`
	ProjectSlug     string `json:"project_slug,omitempty"`
	ClaimBaseSha    string `json:"claim_base_sha,omitempty"`
	ClaimBaseBranch string `json:"claim_base_branch,omitempty"`
}

// scopeInfo mirrors AgentClaimScope from the server, carrying the "why did
// my claim return empty" signal up to the loop supervisor. Populated only
// for scoped claims (--scope-issue); nil for global/unscoped claims.
type scopeInfo struct {
	IssueID      string
	State        string
	Reason       string
	OpenBlockers []string
}

// terminal reports whether the scope subtree has no claimable work and will
// not gain any: the scope issue itself is closed, or every descendant is
// closed/blocked so the tracker is closable. The third stalled reason —
// "all-blocked" — is transient (another worker may release a lease or a
// blocker may close) and is NOT terminal.
func (s *scopeInfo) terminal() bool {
	if s == nil {
		return false
	}
	switch s.State {
	case "closed":
		return true
	case "stalled":
		return s.Reason == "tracker-closable"
	}
	return false
}

// fromAgentClaimBody projects the typed dxclient response into the internal
// (claimedTodo, scopeInfo) pair used by the rest of the lifecycle (take.go,
// manager.go, claude.go). claimedTodo is nil when the response carries no
// real claim — either b is nil (transport-level miss) or b.Id == 0 (server
// returned an empty TodoItem alongside a populated Scope, the wire shape
// for "scope stalled/closed"). scopeInfo is non-nil only for scoped claims.
func fromAgentClaimBody(b *dxclient.AgentClaimBody) (*claimedTodo, *scopeInfo) {
	if b == nil {
		return nil, nil
	}
	var sc *scopeInfo
	if b.Scope != nil {
		sc = &scopeInfo{
			IssueID: b.Scope.IssueId,
			State:   b.Scope.State,
		}
		if b.Scope.Reason != nil {
			sc.Reason = *b.Scope.Reason
		}
		if b.Scope.OpenBlockers != nil {
			sc.OpenBlockers = *b.Scope.OpenBlockers
		}
	}
	// Empty TodoItem (Id==0) means no claim was made — server-side this is
	// how scoped stalls and other no-claim 200 paths surface. Returning a
	// non-nil claimedTodo here would let the loop tight-spin against the
	// empty payload (the original IS-1234 hot-fire).
	if b.Id == 0 {
		return nil, sc
	}
	t := &claimedTodo{
		ID:         b.Id,
		Text:       b.Text,
		Key:        b.Key,
		Kind:       b.Kind,
		TargetType: b.TargetType,
		TargetID:   b.TargetId,
		IssueRef:   b.IssueRef,
		Priority:   b.Priority,
	}
	if b.TargetBranch != nil {
		t.TargetBranch = *b.TargetBranch
	}
	if b.ClaimedBy != nil {
		t.ClaimedBy = *b.ClaimedBy
	}
	if b.ProjectSlug != nil {
		t.ProjectSlug = *b.ProjectSlug
	}
	if b.ClaimBaseSha != nil {
		t.ClaimBaseSha = *b.ClaimBaseSha
	}
	if b.ClaimBaseBranch != nil {
		t.ClaimBaseBranch = *b.ClaimBaseBranch
	}
	return t, sc
}

// claimNextTodo atomically reserves the next available todo for agentID via
// the server's agent claim endpoint (FOR UPDATE SKIP LOCKED on the DB side).
// Returns nil + nil error when no work is available so callers can sleep
// and retry without distinguishing "transport error" from "idle".
//
// Routing: when rc.slug is empty (global agent: srcless or DX_GLOBAL=1) the
// daemon hits /api/dx/agent/claim-any, which picks across every project
// ordered by project.priority then todo.priority. When rc.slug is set the
// daemon stays on the project-scoped /claim path. The wire response shape
// is identical (claim-any populates project_slug; /claim leaves it blank).
func claimNextTodo(rc remoteConfig, agentID, persona string, leaseMinutes int32, scopeIssueID string) (*claimedTodo, *scopeInfo, error) {
	c := cli.NewClient(rc.url, rc.key)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if persona == "" {
		persona = DefaultPersona
	}

	if rc.slug == "" {
		// Global (claim-any) path. scope_issue_id is project-scoped and not
		// supported here — pass it through silently as nil; if an operator
		// supplied --scope-issue on a global agent the loop entry point
		// should already have rejected it (TODO at the cobra layer).
		resp, err := c.AgentClaimAnyWithResponse(ctx, &dxclient.AgentClaimAnyParams{}, dxclient.AgentClaimAnyJSONRequestBody{
			AgentId:      agentID,
			LeaseMinutes: &leaseMinutes,
			Persona:      &persona,
		})
		if err != nil {
			return nil, nil, err
		}
		if resp.StatusCode() >= 400 {
			return nil, nil, fmt.Errorf("claim HTTP %d", resp.StatusCode())
		}
		t, s := fromAgentClaimBody(resp.JSON200)
		return t, s, nil
	}

	body := dxclient.AgentClaimJSONRequestBody{
		Slug:         rc.slug,
		AgentId:      agentID,
		LeaseMinutes: &leaseMinutes,
		Persona:      &persona,
	}
	if scopeIssueID != "" {
		body.ScopeIssueId = &scopeIssueID
	}
	resp, err := c.AgentClaimWithResponse(ctx, &dxclient.AgentClaimParams{}, body)
	if err != nil {
		return nil, nil, err
	}
	if resp.StatusCode() >= 400 {
		return nil, nil, fmt.Errorf("claim HTTP %d", resp.StatusCode())
	}
	t, s := fromAgentClaimBody(resp.JSON200)
	return t, s, nil
}

// renewTodoLease pushes the lease deadline forward so a long-running session
// retains its claim. Best-effort — failures are silent because the next tick
// will retry, and a single missed renewal won't break the session.
func renewTodoLease(rc remoteConfig, todoID int32, agentID string, leaseMinutes int32) {
	c := cli.NewClient(rc.url, rc.key)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, _ = c.AgentRenewWithResponse(ctx, dxclient.AgentRenewJSONRequestBody{
		Id:           todoID,
		AgentId:      agentID,
		LeaseMinutes: &leaseMinutes,
	})
}

type releaseResult struct {
	ChurnDowngraded bool
	CycleDetected   bool
}

// releaseTodo posts the release/resolve call. The server may report:
//   - churn_downgraded: session had no mutations, resolve was downgraded to release
//   - cycle_detected: the resolved todo would be immediately regenerated by the queue,
//     indicating the agent cannot actually fix the underlying condition. Server auto-blocks.
func releaseTodo(rc remoteConfig, todoID int32, agentID, sessionID, claimBaseSHA, todoKind, claimBaseBranch string, resolve bool) releaseResult {
	body := dxclient.AgentReleaseJSONRequestBody{
		Id:        todoID,
		AgentId:   agentID,
		Resolve:   &resolve,
		SessionId: &sessionID,
	}
	if bs := cli.CollectBranchState(claimBaseSHA); bs != nil {
		body.BranchState = bs
		if resolve {
			if err := cli.ValidateBranchState(todoKind, bs, claimBaseSHA, claimBaseBranch); err != nil {
				fmt.Fprintf(os.Stderr, "branch contract violation: %v\ndowngrading resolve to release\n", err)
				downgraded := false
				body.Resolve = &downgraded
			}
		}
	}
	c := cli.NewClient(rc.url, rc.key)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	resp, err := c.AgentReleaseWithResponse(ctx, body)
	if err != nil || resp == nil || resp.JSON200 == nil {
		return releaseResult{}
	}
	r := resp.JSON200
	out := releaseResult{}
	if r.ChurnDowngraded != nil {
		out.ChurnDowngraded = *r.ChurnDowngraded
	}
	if r.CycleDetected != nil {
		out.CycleDetected = *r.CycleDetected
	}
	return out
}
