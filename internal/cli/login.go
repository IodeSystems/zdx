package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	)
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Refresh API credentials by validating a key and writing it locally",
		Long: `Validate an API key against /api/me and persist it for subsequent dx commands.

Without flags, prompts for the key on stdin. With --key, uses the literal value.
With --stdin, reads one whitespace-trimmed line from stdin (suitable for piping).

On success, writes atomically to .zdx/credentials (or ~/.zdx/daemon.token with --daemon).
On rejection, leaves the existing credential file untouched and exits non-zero.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			candidate, err := readCandidateKey(key, stdinFlag)
			if err != nil {
				return err
			}
			if candidate == "" {
				return fmt.Errorf("no API key provided")
			}

			base, err := resolveLoginURL(urlFlag, daemon)
			if err != nil {
				return err
			}

			me, err := validateKey(base, candidate)
			if err != nil {
				return fmt.Errorf("key rejected by %s: %w\n(existing credential file untouched)", base, err)
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
	cmd.Flags().StringVar(&key, "key", "", "API key literal (skips prompt)")
	cmd.Flags().BoolVar(&stdinFlag, "stdin", false, "read API key from stdin (one line, whitespace trimmed)")
	cmd.Flags().StringVar(&urlFlag, "url", "", "override server URL for validation (default: configured remote.url or daemon)")
	cmd.Flags().StringVar(&slug, "slug", "", "update remote.slug in .zdx/config.yaml after login")
	cmd.Flags().BoolVar(&daemon, "daemon", false, "write to ~/.zdx/daemon.token instead of .zdx/credentials")
	return cmd
}

type meResponse struct {
	ID    int32  `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// readCandidateKey resolves the key from --key, --stdin, or interactive prompt.
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
	fmt.Fprint(os.Stderr, "API key: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && err != io.EOF {
		return "", err
	}
	return strings.TrimSpace(line), nil
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
