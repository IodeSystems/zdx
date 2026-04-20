package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
	"net/http/cgi"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (h *Handler) registerGitProxyRoutes() {
	h.Mux.Get("/git/{slug}/info/refs", h.handleGitInfoRefs)
	h.Mux.Post("/git/{slug}/git-upload-pack", h.handleGitUploadPack)
	h.Mux.Post("/git/{slug}/git-receive-pack", h.handleGitReceivePack)
}

// extractGitAPIKey returns the API key token from X-Api-Key or Basic auth password.
func extractGitAPIKey(r *http.Request) string {
	if tok := r.Header.Get("X-Api-Key"); tok != "" {
		return tok
	}
	// git sends Authorization: Basic base64(username:password) — use password as token.
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Basic ") {
		decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
		if err != nil {
			return ""
		}
		parts := strings.SplitN(string(decoded), ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

// gitProxyAuth validates the request and returns routing info.
// When upstreamURL is empty the caller should use the bare repo path instead.
func (h *Handler) gitProxyAuth(w http.ResponseWriter, r *http.Request) (slug string, upstreamURL string, upstreamCreds string, ok bool) {
	slug = chi.URLParam(r, "slug")
	token := extractGitAPIKey(r)
	if token == "" {
		w.Header().Set("WWW-Authenticate", `Basic realm="zdx git"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := h.Q.GetApiKeyByToken(r.Context(), token); err != nil {
		w.Header().Set("WWW-Authenticate", `Basic realm="zdx git"`)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	cfg, err := h.Q.GetProjectProxyConfig(r.Context(), slug)
	if err != nil {
		http.Error(w, "project not found", http.StatusNotFound)
		return
	}
	if !cfg.GitEnabled {
		http.Error(w, "git proxy not enabled for this project", http.StatusForbidden)
		return
	}
	upstreamURL = strings.TrimRight(cfg.UpstreamUrl, "/")
	upstreamCreds = cfg.UpstreamCredentials
	ok = true
	return
}

func (h *Handler) proxyGitRequest(w http.ResponseWriter, r *http.Request, upstreamURL, upstreamCreds, suffix string) {
	target := upstreamURL + suffix
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}

	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, r.Body)
	if err != nil {
		http.Error(w, "proxy error", http.StatusInternalServerError)
		return
	}
	// Forward content-type for POST requests.
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	// Swap auth to upstream credentials.
	if upstreamCreds != "" {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte("x-token:"+upstreamCreds)))
	}
	req.Header.Set("Git-Protocol", r.Header.Get("Git-Protocol"))

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		http.Error(w, "upstream error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, resp.Body)
}

// initBareRepo ensures {reposDir}/{slug}.git exists as a bare git repository.
func initBareRepo(reposDir, slug string) (string, error) {
	repoPath := filepath.Join(reposDir, slug+".git")
	headPath := filepath.Join(repoPath, "HEAD")
	if _, err := os.Stat(headPath); err == nil {
		return repoPath, nil
	}
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "init", "--bare", repoPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", &gitInitError{out: string(out), err: err}
	}
	return repoPath, nil
}

type gitInitError struct {
	out string
	err error
}

func (e *gitInitError) Error() string {
	return "git init --bare failed: " + e.err.Error() + ": " + e.out
}

// serveBareGitRequest handles git smart HTTP protocol via git http-backend for
// a local bare repository. Used when upstream_url is empty.
func (h *Handler) serveBareGitRequest(w http.ResponseWriter, r *http.Request, slug string) {
	if _, err := initBareRepo(h.ReposDir, slug); err != nil {
		http.Error(w, "repo init error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// git http-backend reads PATH_INFO relative to GIT_PROJECT_ROOT.
	// Strip /git/{slug} prefix so PATH_INFO is e.g. /info/refs or /git-upload-pack.
	pathInfo := strings.TrimPrefix(r.URL.Path, "/git/"+slug)
	if pathInfo == "" {
		pathInfo = "/"
	}

	gitBin, err := exec.LookPath("git")
	if err != nil {
		http.Error(w, "git not found on server", http.StatusInternalServerError)
		return
	}

	handler := &cgi.Handler{
		Path: gitBin,
		Args: []string{"http-backend"},
		Env: []string{
			"GIT_PROJECT_ROOT=" + h.ReposDir,
			"GIT_HTTP_EXPORT_ALL=1",
			"REMOTE_ADDR=" + r.RemoteAddr,
		},
	}

	// Rewrite the request so the CGI handler sees the correct PATH_INFO:
	// /{slug}.git/{info/refs|git-upload-pack|git-receive-pack}
	rewritten := r.Clone(r.Context())
	rewrittenURL := *r.URL
	rewrittenURL.Path = "/" + slug + ".git" + pathInfo
	rewritten.URL = &rewrittenURL
	// Auth was already validated; remove to avoid leaking the token to git.
	rewritten.Header = r.Header.Clone()
	rewritten.Header.Del("Authorization")

	handler.ServeHTTP(w, rewritten)
}

func (h *Handler) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	slug, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	if upstreamURL == "" {
		h.serveBareGitRequest(w, r, slug)
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/info/refs")
}

func (h *Handler) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	slug, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	if upstreamURL == "" {
		h.serveBareGitRequest(w, r, slug)
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/git-upload-pack")
}

func (h *Handler) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	slug, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	if upstreamURL == "" {
		h.serveBareGitRequest(w, r, slug)
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/git-receive-pack")
}
