package handlers

import (
	"encoding/base64"
	"io"
	"net/http"
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

func (h *Handler) handleGitInfoRefs(w http.ResponseWriter, r *http.Request) {
	_, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/info/refs")
}

func (h *Handler) handleGitUploadPack(w http.ResponseWriter, r *http.Request) {
	_, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/git-upload-pack")
}

func (h *Handler) handleGitReceivePack(w http.ResponseWriter, r *http.Request) {
	_, upstreamURL, upstreamCreds, ok := h.gitProxyAuth(w, r)
	if !ok {
		return
	}
	h.proxyGitRequest(w, r, upstreamURL, upstreamCreds, "/git-receive-pack")
}
