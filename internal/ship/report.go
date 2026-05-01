package ship

import (
	"context"
	"fmt"
	"os"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

const (
	finalizeSource  = "ship-finalize"
	finalizeLogTail = 4 * 1024
)

// finalizeIssuer is the subset of dxclient.ClientWithResponsesInterface that
// ReportFinalizeFailures needs. Satisfied by *cli.Client and test fakes.
type finalizeIssuer interface {
	ListIssuesWithResponse(ctx context.Context, params *dxclient.ListIssuesParams, reqEditors ...dxclient.RequestEditorFn) (*dxclient.ParsedListIssuesResponse, error)
	AddIssueWithResponse(ctx context.Context, body dxclient.AddIssueJSONRequestBody, reqEditors ...dxclient.RequestEditorFn) (*dxclient.ParsedAddIssueResponse, error)
	AddCommentWithResponse(ctx context.Context, body dxclient.AddCommentJSONRequestBody, reqEditors ...dxclient.RequestEditorFn) (*dxclient.AddCommentResponse, error)
}

// ReportFinalizeFailures iterates results and, for each finalize stage that
// failed, files a project-tracker issue (or appends a comment to an existing
// one). All issuer errors are swallowed and logged to stderr — a reporting
// failure must never break the ship pipeline. issuer nil → no-op.
func ReportFinalizeFailures(ctx context.Context, issuer finalizeIssuer, slug, strategy string, results []StageResult) {
	if issuer == nil {
		return
	}
	for _, r := range results {
		if r.Finalize && r.Status == "failed" {
			reportFinalizeFailure(ctx, issuer, slug, strategy, r)
		}
	}
}

func reportFinalizeFailure(ctx context.Context, issuer finalizeIssuer, slug, strategy string, r StageResult) {
	title := fmt.Sprintf("ship finalize: %s failed on %s", r.Name, strategy)
	logTail := tailBytes(r.Log, finalizeLogTail)
	logBlock := fmt.Sprintf("```\n%s\n```", logTail)

	// Dedup: look for an existing open issue with the same title.
	open := "open"
	listResp, listErr := issuer.ListIssuesWithResponse(ctx, &dxclient.ListIssuesParams{
		Slug:   slug,
		Status: &open,
		Search: &title,
	})
	if listErr == nil && listResp.StatusCode() < 400 && listResp.JSON200 != nil && listResp.JSON200.Issues != nil && len(*listResp.JSON200.Issues) > 0 {
		existing := (*listResp.JSON200.Issues)[0]
		issueID := fmt.Sprintf("IS-%d", existing.Id)
		commentResp, cerr := issuer.AddCommentWithResponse(ctx, dxclient.AddCommentRequest{
			Slug:       slug,
			TargetType: "issue",
			TargetId:   issueID,
			Body:       "Recurrence:\n" + logBlock,
		})
		if cerr != nil {
			fmt.Fprintf(os.Stderr, "ship: comment on %s failed: %v\n", issueID, cerr)
		} else if commentResp.StatusCode() >= 400 {
			fmt.Fprintf(os.Stderr, "ship: comment on %s failed: HTTP %d\n", issueID, commentResp.StatusCode())
		}
		return
	}

	issueType := "bug"
	source := finalizeSource
	autoReady := true
	addResp, aerr := issuer.AddIssueWithResponse(ctx, dxclient.AddIssueRequest{
		Slug:      slug,
		Title:     &title,
		IssueType: &issueType,
		Source:    &source,
		Context:   &logBlock,
		AutoReady: &autoReady,
	})
	if aerr != nil {
		fmt.Fprintf(os.Stderr, "ship: issue create failed for %q: %v\n", r.Name, aerr)
		return
	}
	if addResp.StatusCode() >= 400 {
		fmt.Fprintf(os.Stderr, "ship: issue create failed for %q: HTTP %d\n", r.Name, addResp.StatusCode())
	}
}

// tailBytes returns the last n bytes of s.
func tailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
