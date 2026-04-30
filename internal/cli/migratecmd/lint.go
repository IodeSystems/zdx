package migratecmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

// Defect describes a single lint finding in the migrations directory.
type Defect struct {
	Kind    string // "duplicate" | "missing-pair" | "gap" | "stale-schema"
	Message string
}

const defaultShippedSQL = "schema/shipped.sql"

// LintCmd returns `dx migrate lint`.
func LintCmd() *cobra.Command {
	var dir, schemaFile string
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Detect duplicate NNN_, missing pairs, and sequence gaps",
		RunE: func(cmd *cobra.Command, args []string) error {
			defects := LintMigrations(dir, schemaFile)
			for _, d := range defects {
				fmt.Printf("%s: %s\n", d.Kind, d.Message)
			}
			if len(defects) > 0 {
				return fmt.Errorf("%d defect(s) found", len(defects))
			}
			fmt.Println("ok: no defects found")
			return nil
		},
	}
	cmd.Flags().StringVar(&dir, "dir", defaultMigrationsDir, "migrations directory")
	cmd.Flags().StringVar(&schemaFile, "schema", defaultShippedSQL, "path to shipped.sql for staleness check (empty to skip)")
	return cmd
}

// LintMigrations scans dir for migration defects: duplicate NNN prefixes,
// missing up/down pairs, sequence gaps, and (if schemaFile is non-empty)
// whether schema/shipped.sql is older than the newest migration file.
func LintMigrations(dir string, schemaFile string) []Defect {
	files, err := scanMigrations(dir)
	if err != nil {
		return []Defect{{Kind: "error", Message: err.Error()}}
	}

	// Map NNN -> set of base names (for duplicate detection).
	// Map NNN+name+dir -> true (for pair detection).
	byNNN := map[int]map[string]bool{}
	present := map[string]bool{} // key: "NNN/name/dir"

	for _, f := range files {
		if byNNN[f.NNN] == nil {
			byNNN[f.NNN] = map[string]bool{}
		}
		byNNN[f.NNN][f.Name] = true
		present[pairKey(f.NNN, f.Name, f.Dir)] = true
	}

	var defects []Defect

	// 1. Duplicates: same NNN, multiple base names.
	nnns := sortedKeys(byNNN)
	for _, n := range nnns {
		names := byNNN[n]
		if len(names) < 2 {
			continue
		}
		ordered := sortedStringSet(names)
		defects = append(defects, Defect{
			Kind:    "duplicate",
			Message: fmt.Sprintf("NNN %03d used by multiple migrations: %v", n, ordered),
		})
	}

	// 2. Missing pairs: up without down, or down without up.
	// Collect all (NNN, name) pairs.
	type key struct {
		n    int
		name string
	}
	seen := map[key]bool{}
	for _, f := range files {
		seen[key{f.NNN, f.Name}] = true
	}
	pairKeys := make([]key, 0, len(seen))
	for k := range seen {
		pairKeys = append(pairKeys, k)
	}
	sort.Slice(pairKeys, func(i, j int) bool {
		if pairKeys[i].n != pairKeys[j].n {
			return pairKeys[i].n < pairKeys[j].n
		}
		return pairKeys[i].name < pairKeys[j].name
	})
	for _, k := range pairKeys {
		hasUp := present[pairKey(k.n, k.name, "up")]
		hasDown := present[pairKey(k.n, k.name, "down")]
		if hasUp && !hasDown {
			defects = append(defects, Defect{
				Kind:    "missing-pair",
				Message: fmt.Sprintf("%03d_%s.up.sql has no matching down.sql", k.n, k.name),
			})
		} else if hasDown && !hasUp {
			defects = append(defects, Defect{
				Kind:    "missing-pair",
				Message: fmt.Sprintf("%03d_%s.down.sql has no matching up.sql", k.n, k.name),
			})
		}
	}

	// 3. Gaps: sorted unique NNNs with missing numbers.
	if len(nnns) > 1 {
		for i := 1; i < len(nnns); i++ {
			prev, curr := nnns[i-1], nnns[i]
			if curr-prev > 1 {
				defects = append(defects, Defect{
					Kind:    "gap",
					Message: fmt.Sprintf("sequence gap between %03d and %03d", prev, curr),
				})
			}
		}
	}

	// 4. Stale schema: schema/shipped.sql older than the newest migration file.
	if schemaFile != "" {
		defects = append(defects, checkSchemaStale(dir, schemaFile, files)...)
	}

	return defects
}

// checkSchemaStale returns a stale-schema defect if schemaFile is missing or
// older than the newest migration file in dir.
func checkSchemaStale(dir, schemaFile string, files []migrationFile) []Defect {
	if len(files) == 0 {
		return nil
	}
	// Find the newest migration file mtime.
	var newestMtime int64
	var newestName string
	for _, f := range files {
		info, err := os.Stat(fmt.Sprintf("%s/%s", dir, f.Filename))
		if err != nil {
			continue
		}
		if t := info.ModTime().UnixNano(); t > newestMtime {
			newestMtime = t
			newestName = f.Filename
		}
	}
	if newestMtime == 0 {
		return nil
	}

	schemaInfo, err := os.Stat(schemaFile)
	if os.IsNotExist(err) {
		return []Defect{{
			Kind:    "stale-schema",
			Message: fmt.Sprintf("%s does not exist (run bin/ship to regenerate)", schemaFile),
		}}
	}
	if err != nil {
		return []Defect{{Kind: "stale-schema", Message: fmt.Sprintf("stat %s: %v", schemaFile, err)}}
	}

	if schemaInfo.ModTime().UnixNano() < newestMtime {
		return []Defect{{
			Kind:    "stale-schema",
			Message: fmt.Sprintf("%s is older than %s (run bin/ship to regenerate)", schemaFile, newestName),
		}}
	}
	return nil
}

func pairKey(n int, name, dir string) string {
	return fmt.Sprintf("%d/%s/%s", n, name, dir)
}

func sortedKeys(m map[int]map[string]bool) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func sortedStringSet(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
