// Package preflight implements ship-time pre-flight checks. The migration
// history check ports the bash block at bin/ship:298-377 to Go so the YAML
// ship pipeline (IS-886) and the dx CLI can both invoke it.
package preflight

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// NoteKind enumerates the outcomes for a single shipped migration entry.
// Strings match the human labels used in the bash original so log output is
// byte-identical.
type NoteKind string

const (
	NoteOK              NoteKind = "OK"
	NoteRenamed         NoteKind = "RENAMED"
	NoteRenamedModified NoteKind = "RENAMED+MODIFIED"
	NoteModified        NoteKind = "MODIFIED"
	NoteFirstDeploy     NoteKind = "FIRST_DEPLOY"
)

// Note is a single advisory about a shipped migration. Msg is the human-
// readable text that callers should log verbatim (matches bash NOTE/INFO
// strings).
type Note struct {
	Kind NoteKind
	Msg  string
}

// PendingFix points at a migration-fixes/*.sql file the deploy packaging
// stage must rsync to the host alongside the new migrations.
type PendingFix struct {
	Path string
}

// Options controls MigrationHistory.
type Options struct {
	// DeployedSHA is the commit currently running on the target environment.
	// Caller resolves it (API record or ssh fallback). Empty → first deploy.
	// A "-dirty" suffix is stripped before any git operation.
	DeployedSHA string
	// MigrationDir is the repo-relative directory holding *.sql migrations.
	// Default "internal/migrate/sql".
	MigrationDir string
	// FixesDir is the directory holding manual <base>-<server8>-<dev8>.sql
	// fix files. Default "<MigrationDir>/migration-fixes".
	FixesDir string
}

// MigrationHistory replays the bash pre-flight block in Go. It returns:
//   - notes: one entry per non-trivial outcome (RENAMED, RENAMED+MODIFIED,
//     MODIFIED, FIRST_DEPLOY). Exact filename+content matches yield no Note,
//     matching the bash silent-OK path.
//   - fixes: every acknowledged fix file the caller must package for deploy.
//   - error: non-nil when at least one migration diverged without a fix file.
//     Notes/fixes accumulated before the divergence are still returned so the
//     caller can log a complete picture before refusing.
func MigrationHistory(ctx context.Context, opts Options) ([]Note, []PendingFix, error) {
	if opts.MigrationDir == "" {
		opts.MigrationDir = "internal/migrate/sql"
	}
	if opts.FixesDir == "" {
		opts.FixesDir = filepath.Join(opts.MigrationDir, "migration-fixes")
	}

	deployed := strings.TrimSuffix(opts.DeployedSHA, "-dirty")
	if deployed == "" {
		return []Note{{Kind: NoteFirstDeploy, Msg: "No deployed commit — first deploy, skipping history check."}}, nil, nil
	}

	localIndex, err := buildLocalHashIndex(opts.MigrationDir)
	if err != nil {
		return nil, nil, fmt.Errorf("preflight: index local migrations: %w", err)
	}

	shippedNames, err := listShippedMigrations(ctx, deployed, opts.MigrationDir)
	if err != nil {
		return nil, nil, fmt.Errorf("preflight: list shipped migrations: %w", err)
	}

	var (
		notes       []Note
		fixes       []PendingFix
		divergeMsgs []string
	)

	for _, mig := range shippedNames {
		shippedHash, err := gitShowSha256(ctx, deployed, opts.MigrationDir+"/"+mig)
		if err != nil {
			return notes, fixes, fmt.Errorf("preflight: hash shipped %s: %w", mig, err)
		}
		server8 := shippedHash[:8]
		base := strings.TrimSuffix(mig, ".sql")
		localFile := filepath.Join(opts.MigrationDir, mig)

		localBytes, localErr := os.ReadFile(localFile)
		if os.IsNotExist(localErr) {
			// Local file absent — pure rename or rename+modified.
			if other, ok := localIndex[shippedHash]; ok {
				notes = append(notes, Note{
					Kind: NoteRenamed,
					Msg:  fmt.Sprintf("migration RENAMED: %s → %s (OK)", mig, other),
				})
				continue
			}
			fix, found, err := findFixByPrefix(opts.FixesDir, base, server8)
			if err != nil {
				return notes, fixes, fmt.Errorf("preflight: scan fixes for %s: %w", mig, err)
			}
			if found {
				notes = append(notes, Note{
					Kind: NoteRenamedModified,
					Msg:  fmt.Sprintf("migration RENAMED+MODIFIED (fix acknowledged): %s — %s", mig, filepath.Base(fix)),
				})
				fixes = append(fixes, PendingFix{Path: fix})
				continue
			}
			divergeMsgs = append(divergeMsgs,
				fmt.Sprintf("shipped migration missing locally (not a rename): %s\n  create: %s",
					mig, filepath.Join(opts.FixesDir, fmt.Sprintf("%s-%s-<dev8>.sql", base, server8))))
			continue
		}
		if localErr != nil {
			return notes, fixes, fmt.Errorf("preflight: read %s: %w", localFile, localErr)
		}

		localHash := sha256Hex(localBytes)
		if localHash == shippedHash {
			continue // silent OK — bytes match
		}
		dev8 := localHash[:8]
		fixPath := filepath.Join(opts.FixesDir, fmt.Sprintf("%s-%s-%s.sql", base, server8, dev8))
		if _, err := os.Stat(fixPath); err == nil {
			notes = append(notes, Note{
				Kind: NoteModified,
				Msg:  fmt.Sprintf("migration MODIFIED (fix acknowledged): %s — %s", mig, filepath.Base(fixPath)),
			})
			fixes = append(fixes, PendingFix{Path: fixPath})
			continue
		}
		divergeMsgs = append(divergeMsgs,
			fmt.Sprintf("shipped migration MODIFIED: %s\n  create: %s", mig, fixPath))
	}

	if len(divergeMsgs) > 0 {
		return notes, fixes, fmt.Errorf("%s", strings.Join(divergeMsgs, "\n"))
	}
	return notes, fixes, nil
}

// buildLocalHashIndex maps content-sha256 → basename for every *.sql at the
// top level of dir. Subdirectories (notably migration-fixes/) are skipped.
func buildLocalHashIndex(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	idx := make(map[string]string, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		idx[sha256Hex(data)] = e.Name()
	}
	return idx, nil
}

// listShippedMigrations runs `git ls-tree --name-only <sha> <dir>/` and
// returns the basenames of *.sql entries. Non-sql entries (such as the
// migration-fixes/ subdirectory) are filtered.
func listShippedMigrations(ctx context.Context, sha, dir string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "git", "ls-tree", "--name-only", sha, dir+"/")
	out, err := cmd.Output()
	if err != nil {
		return nil, gitErr(err)
	}
	var names []string
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		base := filepath.Base(line)
		if !strings.HasSuffix(base, ".sql") {
			continue
		}
		names = append(names, base)
	}
	sort.Strings(names) // deterministic ordering for tests and logs
	return names, nil
}

// gitShowSha256 returns the lowercase hex sha256 of `git show <sha>:<path>`.
func gitShowSha256(ctx context.Context, sha, path string) (string, error) {
	out, err := exec.CommandContext(ctx, "git", "show", sha+":"+path).Output()
	if err != nil {
		return "", gitErr(err)
	}
	return sha256Hex(out), nil
}

// findFixByPrefix returns the first *.sql in fixesDir whose name starts with
// "<base>-<server8>-". The bash original used `ls | head -1`; we sort
// lexicographically for determinism.
func findFixByPrefix(fixesDir, base, server8 string) (string, bool, error) {
	pattern := filepath.Join(fixesDir, fmt.Sprintf("%s-%s-*.sql", base, server8))
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", false, err
	}
	if len(matches) == 0 {
		return "", false, nil
	}
	sort.Strings(matches)
	return matches[0], true, nil
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// gitErr unwraps *exec.ExitError to surface stderr in the message.
func gitErr(err error) error {
	if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(ee.Stderr)))
	}
	return err
}
