package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// Client talks to a running dx-server instance.
//
// Client embeds *dxclient.ClientWithResponses so callers get typed methods
// for every endpoint (e.g. c.ListFocusesWithResponse). Auth + attribution
// headers are attached via a RequestEditorFn registered at construction.
// Use c.CheckStatus(resp.StatusCode(), resp.Body) to translate 4xx into
// the authHint-enriched error.
type Client struct {
	*dxclient.ClientWithResponses

	base      string
	token     string
	slug      string
	tokenFrom string // human-readable description of where token came from
	http      *http.Client
}

// Token-source labels surfaced in 401/403 error hints.
const (
	tokenSrcEnv         = "$DX_REMOTE_API_KEY env var"
	tokenSrcCredentials = ".zdx/credentials"
	tokenSrcDaemon      = "~/.zdx/daemon.token"
	tokenSrcNone        = "no credentials configured"
)

// DefaultClient resolves connection in priority order:
//  1. DX_REMOTE_URL env / .zdx/config.yaml remote.url  → explicit server
//  2. ~/.zdx/daemon.{port,token}                        → local daemon
//  3. neither                                           → error
func DefaultClient() (*Client, error) {
	cfg := config.Load()

	base := cfg.RemoteURL()
	slug := cfg.RemoteSlug()
	token, tokenFrom := resolveRemoteAPIKey()

	if base == "" {
		conn := config.ReadDaemonConn()
		if conn == nil {
			return nil, fmt.Errorf("daemon not running — start it with: dx daemon start\n(or set DX_REMOTE_URL / remote.url in .zdx/config.yaml)")
		}
		base = conn.URL
		token = conn.Token
		tokenFrom = tokenSrcDaemon
	}

	c := &Client{
		base:      base,
		token:     token,
		slug:      slug,
		tokenFrom: tokenFrom,
		http:      &http.Client{},
	}
	if err := c.initTyped(); err != nil {
		return nil, err
	}
	return c, nil
}

// NewClient returns a Client talking to base with the given token. The slug
// is empty; callers that need project-scoped operations must supply it via a
// different constructor or fall back to DefaultClient. Used by setup/integrate
// flows that mint tokens before a project is selected.
func NewClient(base, token string) *Client {
	c := &Client{
		base:  base,
		token: token,
		http:  &http.Client{},
	}
	if err := c.initTyped(); err != nil {
		// NewClient signature is (base, token) *Client — an init failure here
		// means a malformed base URL, which is a programmer error at callsites
		// that mint tokens before a project is selected. Fall back to a
		// typed-nil embed; untyped Get/Post still work.
		return c
	}
	return c
}

// initTyped constructs the embedded dxclient.ClientWithResponses and wires
// the auth + attribution request editor.
func (c *Client) initTyped() error {
	tc, err := dxclient.NewClientWithResponses(c.base,
		dxclient.WithHTTPClient(c.http),
		dxclient.WithRequestEditorFn(c.authEditor),
	)
	if err != nil {
		return err
	}
	c.ClientWithResponses = tc
	return nil
}

// authEditor attaches X-Api-Key and agent/session attribution headers to
// every outbound request made via the typed client.
func (c *Client) authEditor(_ context.Context, req *http.Request) error {
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	attachAttributionHeaders(req)
	return nil
}

// resolveRemoteAPIKey mirrors config.RemoteAPIKey but also reports the source.
func resolveRemoteAPIKey() (token, source string) {
	if v := os.Getenv("DX_REMOTE_API_KEY"); v != "" {
		return v, tokenSrcEnv
	}
	if v := config.ReadCredentials(); v != "" {
		return v, tokenSrcCredentials
	}
	return "", tokenSrcNone
}

func (c *Client) Slug() string { return c.slug }

func (c *Client) Get(path string, params url.Values, out any) error {
	u := c.base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, _ := http.NewRequest("GET", u, nil)
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	attachAttributionHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.checkResp(resp, out)
}

// attachAttributionHeaders copies agent/session identifiers from the environment
// onto outbound requests so server-side writes can attribute status changes to
// the invoking agent session. Unset env vars produce no header.
func attachAttributionHeaders(req *http.Request) {
	if v := os.Getenv("ZDX_AGENT_ID"); v != "" {
		req.Header.Set("X-ZDX-Agent-Id", v)
	}
	if v := os.Getenv("ZDX_SESSION_ID"); v != "" {
		req.Header.Set("X-ZDX-Session-Id", v)
	}
}

func (c *Client) Post(path string, body any, out any) error {
	return c.DoJSON("POST", path, body, out)
}

func (c *Client) Put(path string, body any, out any) error {
	return c.DoJSON("PUT", path, body, out)
}

func (c *Client) Patch(path string, body any, out any) error {
	return c.DoJSON("PATCH", path, body, out)
}

// Delete performs an HTTP DELETE. Included because some endpoints accept DELETE
// without a body; callers pass nil body+out.
func (c *Client) Delete(path string, body any, out any) error {
	return c.DoJSON("DELETE", path, body, out)
}

// PostMultipart uploads a file as multipart/form-data. The file bytes are
// attached under the "file" field with the given Content-Type; all other
// entries in fields are string form values. Used by the CLI test-sync path to
// upload demo artifacts (JSON / webm / mp4) to /api/dx/demos/upload.
func (c *Client) PostMultipart(path string, fileField, filename, fileContentType string, fileBody []byte, fields map[string]string, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			return err
		}
	}
	partHeader := make(map[string][]string)
	partHeader["Content-Disposition"] = []string{fmt.Sprintf(`form-data; name=%q; filename=%q`, fileField, filename)}
	partHeader["Content-Type"] = []string{fileContentType}
	part, err := mw.CreatePart(partHeader)
	if err != nil {
		return err
	}
	if _, err := part.Write(fileBody); err != nil {
		return err
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, _ := http.NewRequest("POST", c.base+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	attachAttributionHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.checkResp(resp, out)
}

func (c *Client) DoJSON(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(method, c.base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	attachAttributionHeaders(req)
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return c.checkResp(resp, out)
}

// PostStreamSSE POSTs a JSON body and consumes a text/event-stream response,
// invoking onEvent for each parsed `data:` payload. The callback may return an
// error to stop consumption early.
func (c *Client) PostStreamSSE(ctx context.Context, path string, body any, onEvent func(raw []byte) error) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	attachAttributionHeaders(req)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return c.CheckStatus(resp.StatusCode, data)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = strings.TrimRight(line, "\r\n")
			if strings.HasPrefix(line, "data: ") {
				payload := strings.TrimPrefix(line, "data: ")
				if cbErr := onEvent([]byte(payload)); cbErr != nil {
					return cbErr
				}
			}
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (c *Client) checkResp(resp *http.Response, out any) error {
	body, _ := io.ReadAll(resp.Body)
	if err := c.CheckStatus(resp.StatusCode, body); err != nil {
		return err
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
}

// CheckStatus translates a 4xx/5xx HTTP response into an error matching
// the existing authHint-enriched format. Returns nil for 2xx/3xx.
//
// Callers using the typed dxclient methods invoke this after each request:
//
//	resp, err := c.ListFocusesWithResponse(ctx, params)
//	if err != nil { return err }
//	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil { return err }
//	// use resp.JSON200
func (c *Client) CheckStatus(statusCode int, body []byte) error {
	if statusCode < 400 {
		return nil
	}
	var e struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Error  string `json:"error"`
		Errors []struct {
			Message  string `json:"message"`
			Location string `json:"location"`
			Value    any    `json:"value"`
		} `json:"errors"`
	}
	_ = json.Unmarshal(body, &e)
	msg := e.Title
	if msg == "" {
		msg = e.Error
	}
	if e.Detail != "" {
		msg += ": " + e.Detail
	}
	for _, ve := range e.Errors {
		msg += fmt.Sprintf("\n  - %s at %s (value: %v)", ve.Message, ve.Location, ve.Value)
	}
	if hint := c.authHint(statusCode); hint != "" {
		if msg != "" {
			msg += "\n" + hint
		} else {
			msg = hint
		}
	}
	if msg != "" {
		return fmt.Errorf("HTTP %d: %s", statusCode, msg)
	}
	return fmt.Errorf("HTTP %d", statusCode)
}

// authHint returns a recovery message for 401/403 naming the credential
// source that was used and how to refresh it.
func (c *Client) authHint(status int) string {
	switch status {
	case http.StatusUnauthorized:
		src := c.tokenFrom
		if src == "" {
			src = tokenSrcNone
		}
		return fmt.Sprintf("API key invalid or expired. Source: %s. Run 'dx login' to refresh.", src)
	case http.StatusForbidden:
		slug := c.slug
		if slug == "" {
			slug = "<unset>"
		}
		return fmt.Sprintf("API key valid but lacks access to project '%s'. Run 'dx login --slug=%s' or check role.", slug, slug)
	}
	return ""
}

// SlugOrDie returns the slug or exits with a helpful message.
func (c *Client) SlugOrDie() string {
	if c.slug == "" {
		fmt.Fprintln(os.Stderr, "error: no project slug — set remote.slug in .zdx/config.yaml or DX_REMOTE_SLUG")
		os.Exit(1)
	}
	return c.slug
}
