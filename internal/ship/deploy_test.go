package ship

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// stubPoster captures the last request body and returns the configured status.
type stubPoster struct {
	statusCode int
	captured   *dxclient.CreateEnvironmentDeployRequest
	failNet    bool
}

func (s *stubPoster) CreateEnvironmentDeployWithResponse(ctx context.Context, slug, name string, body dxclient.CreateEnvironmentDeployJSONRequestBody, reqEditors ...dxclient.RequestEditorFn) (*dxclient.CreateEnvironmentDeployResponse, error) {
	if s.failNet {
		return nil, context.DeadlineExceeded
	}
	s.captured = &dxclient.CreateEnvironmentDeployRequest{
		BuildSha:     body.BuildSha,
		BuildBranch:  body.BuildBranch,
		Status:       body.Status,
		DurationSecs: body.DurationSecs,
		Log:          body.Log,
	}
	// Build a minimal *CreateEnvironmentDeployResponse via httptest round-trip.
	b, _ := json.Marshal(map[string]string{"build_sha": body.BuildSha})
	rec := httptest.NewRecorder()
	rec.WriteHeader(s.statusCode)
	rec.Write(b)
	raw := rec.Result()
	raw.Body = http.NoBody

	// Encode body bytes into the response struct directly.
	resp := &dxclient.CreateEnvironmentDeployResponse{}
	// Use the exported HTTPResponse field so StatusCode() returns correctly.
	resp.HTTPResponse = raw
	resp.Body = b
	return resp, nil
}

func results(statuses ...string) []StageResult {
	rs := make([]StageResult, len(statuses))
	for i, st := range statuses {
		rs[i] = StageResult{Name: "stage" + string(rune('A'+i)), Status: st, Duration: time.Second, Log: "log-" + st}
	}
	return rs
}

func TestPostDeployEvent_AllOk(t *testing.T) {
	p := &stubPoster{statusCode: 200}
	err := PostDeployEvent(context.Background(), p, "slug", "dev", "abc123", "main", results("ok", "ok"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.captured == nil {
		t.Fatal("no request captured")
	}
	if *p.captured.Status != "success" {
		t.Errorf("status = %q, want success", *p.captured.Status)
	}
	if p.captured.DurationSecs == nil || *p.captured.DurationSecs != 2 {
		t.Errorf("duration_secs = %v, want 2", p.captured.DurationSecs)
	}
	if p.captured.Log == nil || !strings.Contains(*p.captured.Log, "[ok]") {
		t.Errorf("log missing [ok] marker: %q", *p.captured.Log)
	}
}

func TestPostDeployEvent_OneFailed(t *testing.T) {
	p := &stubPoster{statusCode: 200}
	err := PostDeployEvent(context.Background(), p, "slug", "dev", "abc", "main", results("ok", "failed"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p.captured.Status != "failure" {
		t.Errorf("status = %q, want failure", *p.captured.Status)
	}
}

func TestPostDeployEvent_SkippedCountsAsSuccess(t *testing.T) {
	p := &stubPoster{statusCode: 200}
	err := PostDeployEvent(context.Background(), p, "slug", "dev", "abc", "main", results("ok", "skipped"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *p.captured.Status != "success" {
		t.Errorf("status = %q, want success", *p.captured.Status)
	}
}

func TestPostDeployEvent_LogTruncation(t *testing.T) {
	big := StageResult{Name: "big", Status: "ok", Duration: time.Second, Log: strings.Repeat("x", maxLogBytes+1000)}
	p := &stubPoster{statusCode: 200}
	err := PostDeployEvent(context.Background(), p, "slug", "dev", "abc", "main", []StageResult{big})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.captured.Log == nil {
		t.Fatal("log is nil")
	}
	if len(*p.captured.Log) > maxLogBytes {
		t.Errorf("log len = %d, want <= %d", len(*p.captured.Log), maxLogBytes)
	}
	if !strings.Contains(*p.captured.Log, "[truncated") {
		t.Errorf("log missing truncation marker: %q", (*p.captured.Log)[:100])
	}
}

func TestPostDeployEvent_PostFailureSwallowed(t *testing.T) {
	p := &stubPoster{failNet: true}
	err := PostDeployEvent(context.Background(), p, "slug", "dev", "abc", "main", results("ok"))
	if err != nil {
		t.Fatalf("expected nil (best-effort), got: %v", err)
	}
}
