// Package migratecmd holds CLI subcommands that operate on the embedded
// migration SQL files in internal/migrate/sql — file-system-only, no DB.
package migratecmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

const defaultMigrationsDir = "internal/migrate/sql"

var migrationFileRE = regexp.MustCompile(`^(\d+)_(.+)\.(up|down)\.sql$`)

// migrationFile represents a parsed migration filename.
type migrationFile struct {
	NNN      int    // numeric prefix
	Name     string // base name between NNN_ and .{up,down}.sql
	Dir      string // up | down
	Filename string // the original filename
}

// renamePlan describes one collision-resolving rename of a (up,down) pair.
type renamePlan struct {
	Name   string // base name (shared by up + down)
	FromN  int    // current colliding NNN
	ToN    int    // new free NNN
	OldUp  string // old up filename
	NewUp  string // new up filename
	OldDn  string // old down filename
	NewDn  string // new down filename
}

// RenumberCmd returns `dx migrate renumber [--auto]`.
func RenumberCmd() *cobra.Command {
	var auto bool
	var dir string
	cmd := &cobra.Command{
		Use:   "renumber",
		Short: "Rename colliding NNN_ migration files to next free slots",
		Long: `Scan the migrations directory for files sharing the same numeric prefix
(NNN_) but with different base names. For each colliding base name (other
than the alphabetically first, which keeps its slot), reassign the next
free NNN and rename both the up.sql and down.sql files.

Used by the merge-train runner (IS-652) post-rebase, and by humans when
two branches both add migration NNN_*.sql concurrently.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runRenumber(dir, auto, os.Stdout, os.Stdin)
		},
	}
	cmd.Flags().BoolVar(&auto, "auto", false, "skip interactive confirmation (for merge-train runner)")
	cmd.Flags().StringVar(&dir, "dir", defaultMigrationsDir, "migrations directory")
	return cmd
}

func runRenumber(dir string, auto bool, out io.Writer, in io.Reader) error {
	files, err := scanMigrations(dir)
	if err != nil {
		return err
	}
	plans := planRenumber(files)
	if len(plans) == 0 {
		fmt.Fprintln(out, "no collisions detected")
		return nil
	}

	fmt.Fprintf(out, "found %d colliding migration pair(s):\n", len(plans))
	for _, p := range plans {
		fmt.Fprintf(out, "  %s -> %s\n", p.OldUp, p.NewUp)
		fmt.Fprintf(out, "  %s -> %s\n", p.OldDn, p.NewDn)
	}

	if !auto {
		fmt.Fprint(out, "apply rename? [y/N]: ")
		r := bufio.NewReader(in)
		line, _ := r.ReadString('\n')
		if !strings.EqualFold(strings.TrimSpace(line), "y") {
			fmt.Fprintln(out, "aborted")
			return nil
		}
	}

	if err := applyRenumber(dir, plans); err != nil {
		return err
	}

	// Verify clean.
	files2, err := scanMigrations(dir)
	if err != nil {
		return err
	}
	if remaining := planRenumber(files2); len(remaining) != 0 {
		return fmt.Errorf("collisions remain after renumber (%d) — manual intervention needed", len(remaining))
	}

	fmt.Fprintf(out, "renamed %d pair(s)\n", len(plans))
	return nil
}

// scanMigrations reads every migration filename in dir matching NNN_NAME.{up,down}.sql.
// Files that don't match are silently ignored (subdirs, READMEs, etc.).
func scanMigrations(dir string) ([]migrationFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var out []migrationFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := migrationFileRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		n, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		out = append(out, migrationFile{
			NNN:      n,
			Name:     m[2],
			Dir:      m[3],
			Filename: e.Name(),
		})
	}
	return out, nil
}

// planRenumber returns the rename plans needed to resolve every collision.
// Within each colliding NNN, the alphabetically-first base name keeps its
// slot; every other base name is reassigned to the next free NNN (starting
// from max(all NNN)+1, growing as more renames are scheduled).
func planRenumber(files []migrationFile) []renamePlan {
	// Group by NNN -> set of base names.
	byNNN := map[int]map[string]bool{}
	maxNNN := 0
	for _, f := range files {
		if f.NNN > maxNNN {
			maxNNN = f.NNN
		}
		if byNNN[f.NNN] == nil {
			byNNN[f.NNN] = map[string]bool{}
		}
		byNNN[f.NNN][f.Name] = true
	}

	// Iterate NNNs in ascending order so output is deterministic.
	nnns := make([]int, 0, len(byNNN))
	for n := range byNNN {
		nnns = append(nnns, n)
	}
	sort.Ints(nnns)

	next := maxNNN + 1
	var plans []renamePlan
	for _, n := range nnns {
		names := byNNN[n]
		if len(names) < 2 {
			continue
		}
		// Sort base names alphabetically; first one keeps the slot.
		ordered := make([]string, 0, len(names))
		for name := range names {
			ordered = append(ordered, name)
		}
		sort.Strings(ordered)
		for _, name := range ordered[1:] {
			plans = append(plans, renamePlan{
				Name:  name,
				FromN: n,
				ToN:   next,
				OldUp: fmt.Sprintf("%03d_%s.up.sql", n, name),
				NewUp: fmt.Sprintf("%03d_%s.up.sql", next, name),
				OldDn: fmt.Sprintf("%03d_%s.down.sql", n, name),
				NewDn: fmt.Sprintf("%03d_%s.down.sql", next, name),
			})
			next++
		}
	}
	return plans
}

// applyRenumber executes the rename plans on disk.
func applyRenumber(dir string, plans []renamePlan) error {
	for _, p := range plans {
		oldUp := filepath.Join(dir, p.OldUp)
		newUp := filepath.Join(dir, p.NewUp)
		oldDn := filepath.Join(dir, p.OldDn)
		newDn := filepath.Join(dir, p.NewDn)
		// Both files of the pair must exist (lint should have caught a missing
		// pair, but renumber is defensive — refuse if the down is absent so we
		// don't strand an orphan up.sql at a new number).
		if _, err := os.Stat(oldUp); err != nil {
			return fmt.Errorf("missing %s: %w", oldUp, err)
		}
		if _, err := os.Stat(oldDn); err != nil {
			return fmt.Errorf("missing %s: %w", oldDn, err)
		}
		if err := os.Rename(oldUp, newUp); err != nil {
			return fmt.Errorf("rename %s: %w", p.OldUp, err)
		}
		if err := os.Rename(oldDn, newDn); err != nil {
			return fmt.Errorf("rename %s: %w", p.OldDn, err)
		}
	}
	return nil
}
