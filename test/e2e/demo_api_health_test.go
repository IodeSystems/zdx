package e2e

import (
	"net/http"
	"testing"
)

// TestDemoAPI_HealthReturnsBuildSHA is the demo for spec 59 on feature
// stable-deployments: GET /api/health returns {"status":"ok","build_sha":"<sha>"}
// so operators can probe a deployment without authentication.
func TestDemoAPI_HealthReturnsBuildSHA(t *testing.T) {
	rec := newApiRecorder(t, "health-returns-build-sha")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_health_test.go", Note: "health demo source"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_auth.go", Note: "GET /api/health returns status and build_sha"})
	t.Cleanup(rec.Save)

	var body struct {
		Status   string `json:"status"`
		BuildSHA string `json:"build_sha"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/health", nil, &body))

	if body.Status != "ok" {
		t.Errorf("status: want %q got %q", "ok", body.Status)
	}
	if body.BuildSHA == "" {
		t.Error("build_sha: empty (ldflags wiring lost)")
	}
}
