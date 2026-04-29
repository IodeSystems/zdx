package e2e

import (
	"net/http"
	"testing"
)

// TestDemoAPI_DoctorDeferPersistsServerSide is the demo for spec 131:
// POST /api/dx/doctor/defer persists the deferral server-side, and GET
// /api/dx/doctor/deferrals returns it on subsequent requests — confirming
// deferred checks survive across dx doctor runs without re-prompting.
func TestDemoAPI_DoctorDeferPersistsServerSide(t *testing.T) {
	rec := newApiRecorder(t, "doctor-defer-persists-server-side")
	rec.AddCoderef(coderef{FilePath: "test/e2e/demo_api_doctor_defer_test.go", Note: "demo source (spec 131)"})
	rec.AddCoderef(coderef{FilePath: "internal/server/handlers/handlers_doctor.go", LineStart: 69, LineEnd: 122, Note: "POST /api/dx/doctor/defer + GET /api/dx/doctor/deferrals"})
	rec.AddCoderef(coderef{FilePath: "internal/doctor/doctor.go", LineStart: 201, LineEnd: 208, Note: "evaluateCheck returns StatusDeferred when check is in state.Deferred"})
	rec.AddCoderef(coderef{FilePath: "internal/cli/project/doctor.go", LineStart: 332, LineEnd: 336, Note: "CLI populates state.Deferred from GET /api/dx/doctor/deferrals on startup"})
	t.Cleanup(rec.Save)

	const slug = "demo-doctor-defer"
	mustOK(t, rec.Do(http.MethodPost, "/api/project", map[string]any{
		"slug": slug,
		"name": "Demo Doctor Defer",
	}, nil))

	const checkName = "credentials_exist"

	// POST defer — deferral must be persisted server-side.
	var deferResp struct {
		OK bool `json:"ok"`
	}
	mustOK(t, rec.Do(http.MethodPost, "/api/dx/doctor/defer", map[string]any{
		"slug":       slug,
		"check_name": checkName,
		"rung":       "scaffold",
	}, &deferResp))
	if !deferResp.OK {
		t.Fatalf("defer POST: want ok=true, got false")
	}

	// GET deferrals — deferred check must appear in the list.
	var listResp struct {
		Deferrals []struct {
			CheckName string `json:"check_name"`
			Rung      string `json:"rung"`
		} `json:"deferrals"`
	}
	mustOK(t, rec.Do(http.MethodGet, "/api/dx/doctor/deferrals?slug="+slug, nil, &listResp))

	found := false
	for _, d := range listResp.Deferrals {
		if d.CheckName == checkName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("GET deferrals: check %q not found; got %+v", checkName, listResp.Deferrals)
	}
}
