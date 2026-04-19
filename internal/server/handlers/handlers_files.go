package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerFileRoutes() {
	h.Mux.Post("/api/upload", h.handleUpload)
	h.Mux.Get("/api/files/{id}", h.handleFileServe)
	h.Mux.Get("/api/dx/demos", h.handleListDemos)
	h.Mux.Get("/api/dx/demos/{id}", h.handleGetDemo)
	h.Mux.Get("/api/dx/demos/{id}/console-log", h.handleDemoConsoleLog)
	h.Mux.Get("/api/dx/demos/{type}/{name}", h.handleServeDemo)
	h.Mux.Post("/api/dx/demos/upload", h.handleDemoUpload)
}

// handleUpload accepts multipart/form-data with a single "file" field.
// Stores to h.UploadsDir and records in zdx_files. Returns {id, url}.
func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	const maxSize = 10 << 20 // 10 MB
	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"invalid multipart"}`, http.StatusBadRequest)
		return
	}
	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer f.Close()

	mimeType := fh.Header.Get("Content-Type")
	allowed := map[string]string{
		"image/png":  ".png",
		"image/jpeg": ".jpg",
		"image/gif":  ".gif",
		"image/webp": ".webp",
	}
	ext, ok := allowed[mimeType]
	if !ok {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"unsupported file type"}`, http.StatusBadRequest)
		return
	}

	// Generate a unique filename: year/month/randomhex.ext
	now := time.Now().UTC()
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	relPath := filepath.Join(
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		hex.EncodeToString(rnd[:])+ext,
	)
	absPath := filepath.Join(h.UploadsDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(absPath)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	n, err := io.Copy(dst, f)
	dst.Close()
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	row, err := h.Q.CreateFile(r.Context(), db.CreateFileParams{
		Provider:  "fs",
		Path:      relPath,
		MimeType:  mimeType,
		SizeBytes: n,
	})
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":  row.ID,
		"url": fmt.Sprintf("/api/files/%d", row.ID),
	})
}

// handleFileServe serves an uploaded file by its zdx_files.id.
func (h *Handler) handleFileServe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := h.Q.GetFile(r.Context(), int32(id))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Join(h.UploadsDir, row.Path)
	w.Header().Set("Content-Type", row.MimeType)
	// Inline disposition with a real filename so mobile download managers
	// (Android in particular) save the file with the right extension instead
	// of guessing .txt from the extensionless URL path.
	name := fmt.Sprintf("file-%d%s", id, extensionForMimeType(row.MimeType))
	w.Header().Set("Content-Disposition", contentDispositionInline(name))
	http.ServeFile(w, r, absPath)
}

// ── Demo handlers ────────────────────────────────────────────────────────

func (h *Handler) demosDir() string {
	if d := os.Getenv("DEMOS_DIR"); d != "" {
		return d
	}
	return ".zdx/demo"
}

type DemoListItem struct {
	ID             int32  `json:"id"`
	TestID         int32  `json:"test_id"`
	Type           string `json:"type"`
	Name           string `json:"name"`
	TestComponent  string `json:"test_component"`
	TestName       string `json:"test_name"`
	TestStatus     string `json:"test_status"`
	TestDurationMs int32  `json:"test_duration_ms"`
	URL            string `json:"url"`
	RecordedBranch string `json:"recorded_branch,omitempty"`
	RecordedSha    string `json:"recorded_sha,omitempty"`
}

// handleListDemos returns all demos attached to tests in the given project.
// Prefers /api/files/{id} (file_id set by handleDemoUpload) and falls back to
// the path-based /api/dx/demos/{type}/{name} route for legacy rows whose
// artifacts were never uploaded.
func (h *Handler) handleListDemos(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"slug query param required"}`, http.StatusBadRequest)
		return
	}
	p, err := h.Q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"project not found"}`, http.StatusNotFound)
		return
	}
	rows, err := h.Q.ListDemos(r.Context(), p.ID)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	items := make([]DemoListItem, 0, len(rows))
	for _, row := range rows {
		name := strings.TrimSuffix(filepath.Base(row.ArtifactPath), filepath.Ext(row.ArtifactPath))
		var assetPath string
		if row.FileID.Valid {
			assetPath = fmt.Sprintf("/api/files/%d", row.FileID.Int32)
		} else {
			assetPath = fmt.Sprintf("/api/dx/demos/%s/%s", row.DemoType, name)
		}
		items = append(items, DemoListItem{
			ID:             row.ID,
			Type:           row.DemoType,
			Name:           name,
			TestComponent:  row.TestComponent,
			TestName:       row.TestName,
			TestStatus:     row.TestStatus,
			TestDurationMs: row.TestDurationMs,
			URL:            SignAssetURL(h.WSSecret, assetPath, time.Hour),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"demos": items})
}

func (h *Handler) handleServeDemo(w http.ResponseWriter, r *http.Request) {
	demoType := chi.URLParam(r, "type")
	demoName := chi.URLParam(r, "name")

	if demoType != "cli" && demoType != "video" {
		http.NotFound(w, r)
		return
	}
	if strings.Contains(demoName, "/") || strings.Contains(demoName, "..") {
		http.NotFound(w, r)
		return
	}

	base := h.demosDir()
	var absPath string
	switch demoType {
	case "cli":
		absPath = filepath.Join(base, "cli", demoName+".json")
	case "video":
		// Video files are content-addressed; try with known extensions.
		for _, ext := range []string{".webm", ".mp4"} {
			candidate := filepath.Join(base, "video", demoName+ext)
			if _, err := os.Stat(candidate); err == nil {
				absPath = candidate
				break
			}
		}
	}

	if absPath == "" {
		http.NotFound(w, r)
		return
	}

	var dispExt string
	switch demoType {
	case "cli":
		w.Header().Set("Content-Type", "application/json")
		dispExt = ".json"
	case "video":
		dispExt = filepath.Ext(absPath)
		if dispExt == ".webm" {
			w.Header().Set("Content-Type", "video/webm")
		} else {
			w.Header().Set("Content-Type", "video/mp4")
		}
	}
	// demoName is validated above to contain no '/' or '..'; safe for filename.
	w.Header().Set("Content-Disposition", contentDispositionInline(demoName+dispExt))
	http.ServeFile(w, r, absPath)
}

func (h *Handler) handleDemoConsoleLog(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := h.Q.GetDemoByID(r.Context(), int32(id))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	base := h.demosDir()
	noExt := strings.TrimSuffix(row.ArtifactPath, filepath.Ext(row.ArtifactPath))
	logPath := filepath.Join(base, noExt+".console.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "could not read console log", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

// handleDemoUpload accepts multipart/form-data with fields:
//   - file (required): the demo artifact (cli json / video webm|mp4)
//   - slug (required)
//   - test_name (required)
//   - demo_type (required): "cli" or "video"
//   - test_component (optional, default "e2e"): component used when looking up
//     the test row. Recorder tests prefixed "TestDemo" register under component
//     "demo" (see internal/cli/devtools/test.go), so uploaders from that path
//     must pass test_component=demo.
//   - artifact_path (optional): stable key for the UpsertTestDemo ON CONFLICT
//     clause. When the CLI sync flow has already created a path-only row via
//     SubmitTestResults, passing the same artifact_path here updates that row's
//     file_id instead of inserting a duplicate. Defaults to the randomly
//     generated upload path, preserving pre-existing direct-upload callers.
//
// Stores the file under UploadsDir, creates a zdx_files row, and upserts the
// zdx_test_demos link so /api/dx/specs/demos can return a /api/files/<id> URL.
func (h *Handler) handleDemoUpload(w http.ResponseWriter, r *http.Request) {
	const maxSize = 50 << 20 // 50 MB for video files
	if err := r.ParseMultipartForm(maxSize); err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"invalid multipart"}`, http.StatusBadRequest)
		return
	}

	slug := r.FormValue("slug")
	testName := r.FormValue("test_name")
	demoType := r.FormValue("demo_type")
	testComponent := r.FormValue("test_component")
	if testComponent == "" {
		testComponent = "e2e"
	}
	if slug == "" || testName == "" || demoType == "" {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"slug, test_name, and demo_type are required"}`, http.StatusBadRequest)
		return
	}
	if demoType != "cli" && demoType != "video" {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"demo_type must be cli or video"}`, http.StatusBadRequest)
		return
	}

	f, fh, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"missing file field"}`, http.StatusBadRequest)
		return
	}
	defer f.Close()

	mimeType := fh.Header.Get("Content-Type")
	allowed := map[string]string{
		"application/json": ".json",
		"video/webm":       ".webm",
		"video/mp4":        ".mp4",
	}
	ext, ok := allowed[mimeType]
	if !ok {
		http.Error(w, `{"title":"Bad Request","status":400,"detail":"unsupported file type; expected json, webm, or mp4"}`, http.StatusBadRequest)
		return
	}

	now := time.Now().UTC()
	var rnd [8]byte
	_, _ = rand.Read(rnd[:])
	relPath := filepath.Join(
		"demos",
		fmt.Sprintf("%d", now.Year()),
		fmt.Sprintf("%02d", now.Month()),
		hex.EncodeToString(rnd[:])+ext,
	)
	absPath := filepath.Join(h.UploadsDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	dst, err := os.Create(absPath)
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}
	n, err := io.Copy(dst, f)
	dst.Close()
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	p, err := getProject(ctx, h.Q, slug)
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"project not found"}`, http.StatusNotFound)
		return
	}

	fileRow, err := h.Q.CreateFile(ctx, db.CreateFileParams{
		Provider:  "fs",
		Path:      relPath,
		MimeType:  mimeType,
		SizeBytes: n,
	})
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	test, err := h.Q.GetTest(ctx, db.GetTestParams{
		ProjectID: p.ID,
		Component: testComponent,
		Name:      testName,
	})
	if err != nil {
		http.Error(w, `{"title":"Not Found","status":404,"detail":"test not found"}`, http.StatusNotFound)
		return
	}

	upsertPath := r.FormValue("artifact_path")
	if upsertPath == "" {
		upsertPath = relPath
	}
	_, err = h.Q.UpsertTestDemo(ctx, db.UpsertTestDemoParams{
		TestID:       test.ID,
		DemoType:     demoType,
		ArtifactPath: upsertPath,
		FileID:       pgtype.Int4{Int32: fileRow.ID, Valid: true},
	})
	if err != nil {
		http.Error(w, `{"title":"Internal Server Error","status":500}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"file_id": fileRow.ID,
	})
}

type DemoSpec struct {
	ID          int32  `json:"id"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	FeatureID   int32  `json:"feature_id"`
}

type DemoDetail struct {
	DemoListItem
	Specs []DemoSpec `json:"specs"`
}

func (h *Handler) handleGetDemo(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := h.Q.GetDemoByID(r.Context(), int32(id))
	if err != nil {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimSuffix(filepath.Base(row.ArtifactPath), filepath.Ext(row.ArtifactPath))
	var assetPath string
	if row.FileID.Valid {
		assetPath = fmt.Sprintf("/api/files/%d", row.FileID.Int32)
	} else {
		assetPath = fmt.Sprintf("/api/dx/demos/%s/%s", row.DemoType, name)
	}

	item := DemoListItem{
		ID:             row.ID,
		TestID:         row.TestID,
		Type:           row.DemoType,
		Name:           name,
		TestComponent:  row.TestComponent,
		TestName:       row.TestName,
		TestStatus:     row.TestStatus,
		TestDurationMs: row.TestDurationMs,
		URL:            SignAssetURL(h.WSSecret, assetPath, time.Hour),
		RecordedBranch: row.RecordedBranch,
		RecordedSha:    row.RecordedSha,
	}

	specs, err := h.Q.ListSpecsCoveredByTest(r.Context(), row.TestID)
	if err != nil {
		specs = nil
	}
	demoSpecs := make([]DemoSpec, 0, len(specs))
	for _, s := range specs {
		demoSpecs = append(demoSpecs, DemoSpec{
			ID:          s.ID,
			Description: s.Description,
			Kind:        string(s.Kind),
			FeatureID:   s.FeatureID,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(DemoDetail{DemoListItem: item, Specs: demoSpecs})
}

// extensionForMimeType returns a leading-dot file extension for the given
// MIME type, or ".bin" when unknown. Used to give asset downloads a real
// extension so mobile download managers don't fall back to .txt.
func extensionForMimeType(mimeType string) string {
	switch strings.ToLower(strings.TrimSpace(strings.SplitN(mimeType, ";", 2)[0])) {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "application/json":
		return ".json"
	case "video/webm":
		return ".webm"
	case "video/mp4":
		return ".mp4"
	case "text/plain":
		return ".txt"
	}
	return ".bin"
}

// contentDispositionInline builds a Content-Disposition: inline header value
// with both a quoted-string filename and an RFC 5987 filename* parameter.
// Inline preserves <video src> / <img src> rendering; the filename is only
// consulted when the user explicitly downloads the asset.
func contentDispositionInline(name string) string {
	// Strip characters that don't belong in a quoted filename to keep the
	// header well-formed across browsers.
	safe := strings.Map(func(r rune) rune {
		if r == '"' || r == '\\' || r == '\r' || r == '\n' {
			return -1
		}
		return r
	}, name)
	return fmt.Sprintf(`inline; filename=%q; filename*=UTF-8''%s`, safe, urlPathEscape(safe))
}

// urlPathEscape percent-encodes a filename for the RFC 5987 filename* parameter.
// We hand-roll a minimal encoder rather than pulling net/url for one call site.
func urlPathEscape(s string) string {
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case 'A' <= c && c <= 'Z', 'a' <= c && c <= 'z', '0' <= c && c <= '9',
			c == '-', c == '_', c == '.', c == '~':
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}
