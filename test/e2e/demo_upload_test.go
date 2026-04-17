package e2e

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"
	"testing"
)

// TestDemoUpload_UpdatesExistingDemoRow verifies that /api/dx/demos/upload
// upgrades a test_demo row created via /api/dx/test-results/submit (with
// file_id NULL) to point at the uploaded file, so /api/dx/specs/demos returns
// a /api/files/<id> URL instead of the path-based /api/dx/demos/<type>/<name>
// URL that 404s on production.
func TestDemoUpload_UpdatesExistingDemoRow(t *testing.T) {
	requiresAPI(t)

	const slug = "e2e-demoupload"
	apiDo(t, http.MethodPost, "/api/project",
		map[string]string{"slug": slug, "name": "E2E Demo Upload"}, nil)

	// Submit a test result with a demo_artifacts ref. Creates zdx_tests and
	// zdx_test_demos (file_id NULL).
	const testComponent = "demo"
	const testName = "TestDemoCLI_Upload"
	const demoType = "cli"
	const artifactPath = ".zdx/demo/cli/e2e-upload.json"
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/test-results/submit",
		map[string]any{
			"slug": slug,
			"results": []map[string]any{{
				"driver":      testComponent,
				"test_name":   testName,
				"status":      "pass",
				"feature":     "upload-test",
				"duration_ms": 0,
				"demo_artifacts": []map[string]string{{
					"demo_type":     demoType,
					"artifact_path": artifactPath,
				}},
			}},
		}, nil))

	// Look up the test so we can link a spec to it.
	var tests struct {
		Tests []struct {
			ID        int32  `json:"id"`
			Component string `json:"component"`
			Name      string `json:"name"`
		} `json:"tests"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/tests?slug="+slug+"&limit=100", nil, &tests))
	var testID int32
	for _, tr := range tests.Tests {
		if tr.Name == testName && tr.Component == testComponent {
			testID = tr.ID
			break
		}
	}
	if testID == 0 {
		t.Fatalf("test %s/%s not found after submit", testComponent, testName)
	}

	// Create a feature + spec and link the spec to the test so list-spec-demos has
	// something to return.
	mustOK(t, apiDo(t, http.MethodPost, "/api/feature",
		map[string]string{"slug": slug, "name": "upload-test", "description": "feature for demo upload test"}, nil))
	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/update",
		map[string]string{"slug": slug, "feature": "upload-test", "field": "must", "value": "upload demo covers this spec"}, nil))

	// Find the spec id (get-feature returns specs inline).
	var feat struct {
		Specs []struct {
			ID int32 `json:"id"`
		} `json:"specs"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/feature?slug="+slug+"&name=upload-test", nil, &feat))
	if len(feat.Specs) == 0 {
		t.Fatalf("spec not found on feature upload-test")
	}
	specID := feat.Specs[0].ID

	mustOK(t, apiDo(t, http.MethodPost, "/api/dx/specs/link-test",
		map[string]any{"spec_id": specID, "test_id": testID}, nil))

	// Pre-condition: spec-demos already returns the path-only URL.
	var preDemos struct {
		Demos []struct {
			URL string `json:"url"`
		} `json:"demos"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/specs/demos?spec_id="+strconv.Itoa(int(specID)), nil, &preDemos))
	if len(preDemos.Demos) != 1 {
		t.Fatalf("want 1 demo before upload, got %d", len(preDemos.Demos))
	}
	if !strings.Contains(preDemos.Demos[0].URL, "/api/dx/demos/cli/") {
		t.Errorf("pre-upload URL should be path-based, got %q", preDemos.Demos[0].URL)
	}

	// Upload the demo file with artifact_path matching the ingested row so the
	// UpsertTestDemo ON CONFLICT clause updates file_id instead of inserting a
	// duplicate row.
	uploadDemo(t, slug, testComponent, testName, demoType, artifactPath, []byte(`{"steps":[]}`))

	// Post-condition: the URL now routes through /api/files/<id>.
	var postDemos struct {
		Demos []struct {
			URL string `json:"url"`
		} `json:"demos"`
	}
	mustOK(t, apiDo(t, http.MethodGet, "/api/dx/specs/demos?spec_id="+strconv.Itoa(int(specID)), nil, &postDemos))
	if len(postDemos.Demos) != 1 {
		t.Fatalf("want 1 demo after upload (ON CONFLICT update, not insert), got %d: %+v", len(postDemos.Demos), postDemos.Demos)
	}
	if !strings.Contains(postDemos.Demos[0].URL, "/api/files/") {
		t.Errorf("post-upload URL should be file-id-based, got %q", postDemos.Demos[0].URL)
	}
}

func uploadDemo(t *testing.T, slug, component, testName, demoType, artifactPath string, body []byte) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fields := map[string]string{
		"slug":           slug,
		"test_component": component,
		"test_name":      testName,
		"demo_type":      demoType,
		"artifact_path":  artifactPath,
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	header := textproto.MIMEHeader{}
	header.Set("Content-Disposition", `form-data; name="file"; filename="demo.json"`)
	header.Set("Content-Type", "application/json")
	part, err := mw.CreatePart(header)
	if err != nil {
		t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(body); err != nil {
		t.Fatalf("write part: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/dx/demos/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("X-Api-Key", srv.AdminToken)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload %d: %s", resp.StatusCode, string(b))
	}
	var out struct {
		OK     bool  `json:"ok"`
		FileID int32 `json:"file_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.OK || out.FileID == 0 {
		t.Fatalf("upload response: %+v", out)
	}
}
