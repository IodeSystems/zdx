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
	base  string
	token string
	slug  string
	http  *http.Client
}

// DefaultClient resolves connection in priority order:
//  1. DX_REMOTE_URL env / .zdx/config.yaml remote.url  → explicit server
//  2. ~/.zdx/daemon.{port,token}                        → local daemon
//  3. neither                                           → error
func DefaultClient() (*Client, error) {
	cfg := config.Load()

	base := cfg.RemoteURL()
	slug := cfg.RemoteSlug()
	token := config.RemoteAPIKey()

	if base == "" {
		conn := config.ReadDaemonConn()
		if conn == nil {
			return nil, fmt.Errorf("daemon not running — start it with: dx daemon start\n(or set DX_REMOTE_URL / remote.url in .zdx/config.yaml)")
		}
		base = conn.URL
		token = conn.Token
	}

	return &Client{
		base:  base,
		token: token,
		slug:  slug,
		http:  &http.Client{},
	}, nil
}

func (c *Client) Slug() string { return c.slug }

func (c *Client) get(path string, params url.Values, out any) error {
	u := c.base + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, _ := http.NewRequest("GET", u, nil)
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkResp(resp, out)
}

func (c *Client) post(path string, body any, out any) error {
	return c.doJSON("POST", path, body, out)
}

func (c *Client) put(path string, body any, out any) error {
	return c.doJSON("PUT", path, body, out)
}

func (c *Client) patch(path string, body any, out any) error {
	return c.doJSON("PATCH", path, body, out)
}

func (c *Client) doJSON(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(method, c.base+path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("X-Api-Key", c.token)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return checkResp(resp, out)
}

func checkResp(resp *http.Response, out any) error {
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		var e struct {
			Title  string `json:"title"`
			Detail string `json:"detail"`
			Error  string `json:"error"`
		}
		_ = json.Unmarshal(body, &e)
		msg := e.Title
		if msg == "" {
			msg = e.Error
		}
		if e.Detail != "" {
			msg += ": " + e.Detail
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

// SlugOrDie returns the slug or exits with a helpful message.
func (c *Client) SlugOrDie() string {
	if c.slug == "" {
		fmt.Fprintln(os.Stderr, "error: no project slug — set remote.slug in .zdx/config.yaml or DX_REMOTE_SLUG")
		os.Exit(1)
	}
	return c.slug
}
