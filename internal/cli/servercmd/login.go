package servercmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/config"
)

func LoginCmd() *cobra.Command {
	var (
		key       string
		stdinFlag bool
		urlFlag   string
		slug      string
		daemon    bool
		global    bool
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate against a zdx server and persist credentials",
		Long: `Authenticate against a zdx server and persist credentials for subsequent dx commands.

Without --global: validates the provided API key and writes to .zdx/credentials.
With --global --url <url>: runs a browser-based flow (or falls back to manual entry)
and writes to ~/.zdx/credentials + remote.url in ~/.zdx/config.yaml.

Key input modes: --key <literal>, --stdin (one line), or interactive prompt.
The browser flow is skipped when --key or --stdin is provided.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if global && urlFlag == "" {
				return fmt.Errorf("--global requires --url")
			}

			base, err := resolveLoginURL(urlFlag, daemon)
			if err != nil {
				return err
			}

			// Determine the key: explicit flag/stdin, or browser/prompt flow.
			candidate, err := readCandidateKey(key, stdinFlag)
			if err != nil {
				return err
			}

			if candidate == "" {
				if global {
					candidate, err = browserAuthFlow(base)
					if err != nil {
						return err
					}
				} else {
					// Interactive prompt
					fmt.Fprint(os.Stderr, "API key: ")
					line, rerr := bufio.NewReader(os.Stdin).ReadString('\n')
					if rerr != nil && rerr != io.EOF {
						return rerr
					}
					candidate = strings.TrimSpace(line)
				}
			}
			if candidate == "" {
				return fmt.Errorf("no API key provided")
			}

			me, err := validateKey(base, candidate)
			if err != nil {
				return fmt.Errorf("key rejected by %s: %w\n(existing credential file untouched)", base, err)
			}

			if global {
				if err := config.WriteGlobalCredentials(candidate); err != nil {
					return fmt.Errorf("write ~/.zdx/credentials: %w", err)
				}
				cfg := config.LoadGlobal()
				if cfg == nil {
					cfg = &config.GlobalConfig{}
				}
				cfg.Remote.URL = strings.TrimRight(base, "/")
				if err := config.WriteGlobal(cfg); err != nil {
					return fmt.Errorf("write ~/.zdx/config.yaml: %w", err)
				}
				home, _ := os.UserHomeDir()
				fmt.Printf("Logged in as %s (%s)\n", me.Email, me.Role)
				fmt.Printf("Credentials saved to %s\n", filepath.Join(home, ".zdx", "credentials"))
				fmt.Printf("Server URL saved to %s\n", filepath.Join(home, ".zdx", "config.yaml"))
				return nil
			}

			dest, err := credentialPath(daemon)
			if err != nil {
				return err
			}
			if err := writeCredentialAtomic(dest, candidate); err != nil {
				return fmt.Errorf("write %s: %w", dest, err)
			}

			if slug != "" && !daemon {
				writeRemoteConfig(strings.TrimRight(base, "/"), slug)
			}

			fmt.Printf("Logged in as %s (%s)\n", me.Email, me.Role)
			fmt.Printf("Saved to %s\n", dest)
			return nil
		},
	}
	cmd.Flags().StringVar(&key, "key", "", "API key literal (skips prompt and browser flow)")
	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "read API key from stdin (one line, whitespace trimmed)")
	cmd.Flags().StringVar(&urlFlag, "url", "", "server URL (required with --global; otherwise defaults to configured remote.url or daemon)")
	cmd.Flags().StringVar(&slug, "slug", "", "update remote.slug in .zdx/config.yaml after login")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "write to ~/.zdx/daemon.token instead of .zdx/credentials")
	cmd.Flags().BoolVar(&global, "global", false, "save credentials globally to ~/.zdx/credentials and ~/.zdx/config.yaml")
	return cmd
}

// browserAuthFlow starts a local HTTP callback server, opens the browser to the
// /cli-auth page, and waits up to 2 minutes for the token callback.
// Falls back to a manual prompt if the browser cannot be opened or the timeout expires.
func browserAuthFlow(base string) (string, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return manualTokenPrompt(base), nil
	}
	port := ln.Addr().(*net.TCPAddr).Port

	tokenCh := make(chan string, 1)
	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := r.URL.Query().Get("token")
			if token == "" {
				http.Error(w, "missing token", http.StatusBadRequest)
				return
			}
			fmt.Fprintln(w, "<html><body><p>CLI authorized. You may close this tab.</p></body></html>")
			select {
			case tokenCh <- token:
			default:
			}
		}),
	}
	go srv.Serve(ln) //nolint:errcheck

	callbackURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)
	authURL := fmt.Sprintf("%s/code?callback=%s", strings.TrimRight(base, "/"), callbackURL)

	fmt.Fprintf(os.Stderr, "Opening browser for authentication...\n")
	fmt.Fprintf(os.Stderr, "If the browser doesn't open, visit:\n  %s\n\n", authURL)

	_ = openBrowser(authURL)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	defer srv.Shutdown(context.Background()) //nolint:errcheck

	select {
	case token := <-tokenCh:
		return token, nil
	case <-ctx.Done():
		fmt.Fprintln(os.Stderr, "\nBrowser flow timed out. Falling back to manual entry.")
		return manualTokenPrompt(base), nil
	}
}

// manualTokenPrompt prints the auth URL and prompts the user to paste the token.
func manualTokenPrompt(base string) string {
	fmt.Fprintf(os.Stderr, "Visit the following URL to get a token, then paste it here:\n  %s/code\n\nToken: ", strings.TrimRight(base, "/"))
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return ""
	}
	return strings.TrimSpace(line)
}

// openBrowser opens url in the system default browser.
func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

type meResponse struct {
	ID    int32  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// readCandidateKey resolves the key from --key or --stdin; returns "" for interactive.
func readCandidateKey(literal string, fromStdin bool) (string, error) {
	if literal != "" {
		return strings.TrimSpace(literal), nil
	}
	if fromStdin {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(string(b)), nil
	}
	return "", nil
}

// resolveLoginURL picks the validation target.
func resolveLoginURL(override string, daemon bool) (string, error) {
	if override != "" {
		return strings.TrimRight(override, "/"), nil
	}
	if daemon {
		conn := config.ReadDaemonConn()
		if conn == nil {
			return "", fmt.Errorf("--daemon: no running daemon found (~/.zdx/daemon.port missing)")
		}
		return conn.URL, nil
	}
	cfg := config.Load()
	if u := cfg.RemoteURL(); u != "" {
		return strings.TrimRight(u, "/"), nil
	}
	conn := config.ReadDaemonConn()
	if conn != nil {
		return conn.URL, nil
	}
	return "", fmt.Errorf("no server URL: set remote.url in .zdx/config.yaml, DX_REMOTE_URL, or pass --url")
}

// validateKey hits GET /api/me and returns the user record on success.
func validateKey(base, token string) (*meResponse, error) {
	req, _ := http.NewRequest(http.MethodGet, base+"/api/me", nil)
	req.Header.Set("X-Api-Key", token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("unauthorized")
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var me meResponse
	if err := json.Unmarshal(body, &me); err != nil {
		return nil, fmt.Errorf("decode /api/me: %w", err)
	}
	if me.Email == "" {
		return nil, fmt.Errorf("/api/me returned empty email — endpoint may not be the expected zdx server")
	}
	return &me, nil
}

// credentialPath returns the destination file for the new key.
func credentialPath(daemon bool) (string, error) {
	if daemon {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".zdx", "daemon.token"), nil
	}
	return ".zdx/credentials", nil
}

// writeCredentialAtomic writes token to dest via tmp+rename with 0600 perms.
func writeCredentialAtomic(dest, token string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dest), filepath.Base(dest)+".*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.WriteString(strings.TrimSpace(token) + "\n"); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dest)
}
