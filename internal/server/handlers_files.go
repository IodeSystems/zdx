package server

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

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerFileRoutes() {
	s.mux.Post("/api/upload", s.handleUpload)
	s.mux.Get("/api/files/{id}", s.handleFileServe)
	s.mux.Get("/api/dx/demos", s.handleListDemos)
	s.mux.Get("/api/dx/demos/{type}/{name}", s.handleServeDemo)
}

// handleUpload accepts multipart/form-data with a single "file" field.
// Stores to s.uploadsDir and records in zdx_files. Returns {id, url}.
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
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
	absPath := filepath.Join(s.uploadsDir, relPath)
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

	row, err := s.q.CreateFile(r.Context(), db.CreateFileParams{
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
func (s *Server) handleFileServe(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.ParseInt(idStr, 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	row, err := s.q.GetFile(r.Context(), int32(id))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	absPath := filepath.Join(s.uploadsDir, row.Path)
	w.Header().Set("Content-Type", row.MimeType)
	http.ServeFile(w, r, absPath)
}

// ── Demo handlers ────────────────────────────────────────────────────────

func (s *Server) demosDir() string {
	if d := os.Getenv("DEMOS_DIR"); d != "" {
		return d
	}
	return ".zdx/demo"
}

type DemoListItem struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

func (s *Server) handleListDemos(w http.ResponseWriter, r *http.Request) {
	base := s.demosDir()
	var items []DemoListItem

	for _, subdir := range []string{"cli", "video"} {
		dir := filepath.Join(base, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
			items = append(items, DemoListItem{Type: subdir, Name: name})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"demos": items})
}

func (s *Server) handleServeDemo(w http.ResponseWriter, r *http.Request) {
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

	base := s.demosDir()
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

	switch demoType {
	case "cli":
		w.Header().Set("Content-Type", "application/json")
	case "video":
		ext := filepath.Ext(absPath)
		if ext == ".webm" {
			w.Header().Set("Content-Type", "video/webm")
		} else {
			w.Header().Set("Content-Type", "video/mp4")
		}
	}
	http.ServeFile(w, r, absPath)
}
