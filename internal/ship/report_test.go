package ship

import (
	"context"
	"errors"
	"testing"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// stubIssuer records calls to the three finalizeIssuer methods.
type stubIssuer struct {
	// listResult is returned by ListIssuesWithResponse (nil → empty list).
	listResult *dxclient.ListIssuesResponse
	listErr    error

	// addResult is returned by AddIssueWithResponse.
	addErr error
	addID  int32 // id stuffed into the JSON200 response

	// commentErr is returned by AddCommentWithResponse.
	commentErr error

	// recorded calls
	issuesCreated  int
	commentsAdded  int
	lastIssueTitle string
	lastCommentID  string
}

func (s *stubIssuer) ListIssuesWithResponse(_ context.Context, _ *dxclient.ListIssuesParams, _ ...dxclient.RequestEditorFn) (*dxclient.ParsedListIssuesResponse, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	resp := &dxclient.ParsedListIssuesResponse{}
	resp.JSON200 = s.listResult
	return resp, nil
}

func (s *stubIssuer) AddIssueWithResponse(_ context.Context, body dxclient.AddIssueJSONRequestBody, _ ...dxclient.RequestEditorFn) (*dxclient.ParsedAddIssueResponse, error) {
	if s.addErr != nil {
		return nil, s.addErr
	}
	s.issuesCreated++
	if body.Title != nil {
		s.lastIssueTitle = *body.Title
	}
	resp := &dxclient.ParsedAddIssueResponse{}
	resp.JSON200 = &dxclient.AddIssueResponse{Id: s.addID}
	return resp, nil
}

func (s *stubIssuer) AddCommentWithResponse(_ context.Context, body dxclient.AddCommentJSONRequestBody, _ ...dxclient.RequestEditorFn) (*dxclient.AddCommentResponse, error) {
	if s.commentErr != nil {
		return nil, s.commentErr
	}
	s.commentsAdded++
	s.lastCommentID = body.TargetId
	resp := &dxclient.AddCommentResponse{}
	return resp, nil
}

// TestReportFinalizeFailures_NilIssuerIsNoop verifies that a nil issuer
// produces no panic or side-effect.
func TestReportFinalizeFailures_NilIssuerIsNoop(t *testing.T) {
	results := []StageResult{{Name: "post", Status: "failed", Finalize: true}}
	ReportFinalizeFailures(context.Background(), nil, "slug", "simple", results)
}

// TestReportFinalizeFailures_CreatesIssue verifies that a failed finalize
// stage with no existing open issue creates exactly one new issue with the
// correct title format and source.
func TestReportFinalizeFailures_CreatesIssue(t *testing.T) {
	empty := []dxclient.IssueItem{}
	stub := &stubIssuer{
		listResult: &dxclient.ListIssuesResponse{Issues: &empty},
		addID:      42,
	}
	results := []StageResult{
		{Name: "promote-shipped", Status: "failed", Log: "some log output", Finalize: true},
	}
	ReportFinalizeFailures(context.Background(), stub, "myslug", "rolling-pair", results)

	if stub.issuesCreated != 1 {
		t.Errorf("issues created = %d, want 1", stub.issuesCreated)
	}
	want := "ship finalize: promote-shipped failed on rolling-pair"
	if stub.lastIssueTitle != want {
		t.Errorf("issue title = %q, want %q", stub.lastIssueTitle, want)
	}
}

// TestReportFinalizeFailures_Dedup verifies that when an open issue already
// exists with the same title, a comment is added instead of creating a new issue.
func TestReportFinalizeFailures_Dedup(t *testing.T) {
	existing := []dxclient.IssueItem{{Id: 99, Title: "ship finalize: promote-shipped failed on simple"}}
	stub := &stubIssuer{
		listResult: &dxclient.ListIssuesResponse{Issues: &existing},
	}
	results := []StageResult{
		{Name: "promote-shipped", Status: "failed", Log: "recurrence log", Finalize: true},
	}
	ReportFinalizeFailures(context.Background(), stub, "myslug", "simple", results)

	if stub.issuesCreated != 0 {
		t.Errorf("issues created = %d, want 0 (dedup should comment)", stub.issuesCreated)
	}
	if stub.commentsAdded != 1 {
		t.Errorf("comments added = %d, want 1", stub.commentsAdded)
	}
	if stub.lastCommentID != "IS-99" {
		t.Errorf("comment target = %q, want IS-99", stub.lastCommentID)
	}
}

// TestReportFinalizeFailures_IssuerErrorSwallowed verifies that when the
// issuer returns an error, ReportFinalizeFailures still completes without
// panicking or returning an error.
func TestReportFinalizeFailures_IssuerErrorSwallowed(t *testing.T) {
	stub := &stubIssuer{
		listErr: errors.New("network error"),
		addErr:  errors.New("also failed"),
	}
	results := []StageResult{
		{Name: "post", Status: "failed", Log: "err log", Finalize: true},
	}
	// Must not panic; no way to check for nil return (function is void).
	ReportFinalizeFailures(context.Background(), stub, "myslug", "simple", results)

	// list failed → fell through to create → create also failed → swallowed
	if stub.issuesCreated != 0 {
		t.Errorf("issues created = %d, want 0 (create errored)", stub.issuesCreated)
	}
}

// TestReportFinalizeFailures_SkipsNonFinalizeResults verifies that main stage
// failures (Finalize=false) are not reported.
func TestReportFinalizeFailures_SkipsNonFinalizeResults(t *testing.T) {
	stub := &stubIssuer{}
	results := []StageResult{
		{Name: "main", Status: "failed", Finalize: false},
		{Name: "post", Status: "ok", Finalize: true},
	}
	ReportFinalizeFailures(context.Background(), stub, "myslug", "simple", results)

	if stub.issuesCreated != 0 || stub.commentsAdded != 0 {
		t.Errorf("unexpected calls: issues=%d comments=%d", stub.issuesCreated, stub.commentsAdded)
	}
}
