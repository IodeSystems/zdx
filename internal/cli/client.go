package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/iodesystems/zdx-go/internal/config"
)

// Client talks to a running dx-server instance.
type Client struct {
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

	return &Client{
		base:      base,
		token:     token,
		slug:      slug,
		tokenFrom: tokenFrom,
		http:      &http.Client{},
	}, nil
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

func (c *Client) checkResp(resp *http.Response, out any) error {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
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
		if hint := c.authHint(resp.StatusCode); hint != "" {
			if msg != "" {
				msg += "\n" + hint
			} else {
				msg = hint
			}
		}
		if msg != "" {
			return fmt.Errorf("HTTP %d: %s", resp.StatusCode, msg)
		}
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(body, out)
	}
	return nil
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
