package work

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

// MergeTrainCmd returns the `dx merge-train` subcommand group.
func MergeTrainCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "merge-train",
		Short: "Rebase worker branch, regen artifacts, lint, and merge into dev",
	}
	cmd.AddCommand(mergeTrainRunCmd())
	return cmd
}

func mergeTrainRunCmd() *cobra.Command {
	var dryRun bool
	var workerPrefix string
	cmd := &cobra.Command{
		Use:   "run [branch]",
		Short: "Rebase, regen, lint, and fast-forward merge a worker branch into dev",
		Long: `Rebase [branch] onto dev, run IS-651 migration-collision renumber, rebuild
binaries, regenerate all codegen artifacts (sqlc, dxclient, openapi-typescript,
schema/shipped.sql when new migrations are present), commit any regen output,
gate on full bin/lint, then fast-forward merge into dev.

Forward-only: rebase conflicts surface for manual resolution and stop; the
branch is left in its pre-rebase state. Re-running on an already-merged branch
is a no-op.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branch := ""
			if len(args) > 0 {
				branch = args[0]
			}
			return mergeTrainRun(branch, dryRun, workerPrefix)
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "print plan without executing")
	cmd.Flags().StringVar(&workerPrefix, "worker-prefix", "agent/", "branch prefix for auto-selection")
	return cmd
}

func mergeTrainRun(branch string, dryRun bool, workerPrefix string) error {
	// 1. Branch resolution.
	if branch == "" {
		var err error
		branch, err = pickWorkerBranch(workerPrefix)
		if err != nil {
			return err
		}
	} else {
		if out, err := exec.Command("git", "rev-parse", "--verify", branch).CombinedOutput(); err != nil {
			return fmt.Errorf("branch %q not found: %s", branch, strings.TrimSpace(string(out)))
		}
	}

	// 2. Idempotency: already an ancestor of dev → no-op.
	if exec.Command("git", "merge-base", "--is-ancestor", branch, "dev").Run() == nil {
		fmt.Printf("%s already merged into dev — no-op\n", branch)
		return nil
	}

	// 3. Print plan (always, even in non-dry-run).
	steps := []string{
		"fetch origin (if remote exists)",
		"git checkout " + branch + " && git rebase dev",
		"dx migrate renumber --auto (if NNN collision)",
		"go build ./cmd/dx ./cmd/dx-server",
		"bin/regen-schema (if new migrations on branch)",
		"sqlc generate",
		"dx ui gen-api",
		"git commit regen output (if dirty)",
		"bin/lint (full mode — no --intent)",
		"git checkout dev && git merge --ff-only " + branch,
	}
	fmt.Printf("merge-train: %s\n", branch)
	for _, s := range steps {
		fmt.Printf("  → %s\n", s)
	}

	if dryRun {
		fmt.Println("(dry-run — not executing)")
		return nil
	}

	root, err := cli.GitRepoRoot()
	if err != nil {
		return err
	}

	// 4. Rebase branch onto dev.
	if err := mtRebase(branch); err != nil {
		return err
	}

	// 5. Migration renumber.
	if err := mtMigrationRenumber(root); err != nil {
		return err
	}

	// 6. Build binaries (embed new migrations; get current OpenAPI spec).
	fmt.Println("merge-train: building binaries...")
	if err := cli.RunShell("go build -o bin/dx ./cmd/dx && go build -o bin/dx-server ./cmd/dx-server", root); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// 7. Regen artifacts.
	if err := mtRegen(root, branch); err != nil {
		return err
	}

	// 8. Commit regen output if dirty.
	if err := mtCommitRegen(branch); err != nil {
		return err
	}

	// 9. Lint gate (full mode — workers ran --intent; merge-train enforces the real gate).
	fmt.Println("merge-train: running bin/lint...")
	if err := cli.RunShell("./bin/lint", root); err != nil {
		return fmt.Errorf("lint failed on %s — fix and re-run", branch)
	}

	// 10. Fast-forward merge into dev.
	if err := mtFFMerge(branch); err != nil {
		return err
	}

	// Summary line.
	tip, _ := mtGitOutput("git", "rev-parse", "--short", "HEAD")
	regenSHA := "none"
	if sha, err := mtGitOutput("git", "log", "--format=%h", "--grep=^regen: "+branch+"$", "HEAD", "-1"); err == nil && sha != "" {
		regenSHA = sha
	}
	fmt.Printf("merged %s into dev (regen=%s, dev tip=%s)\n", branch, regenSHA, tip)
	return nil
}

// pickWorkerBranch returns the first local branch matching prefix that has
// unmerged commits relative to dev.
func pickWorkerBranch(prefix string) (string, error) {
	out, err := exec.Command("git", "branch", "--list", prefix+"*").Output()
	if err != nil {
		return "", fmt.Errorf("git branch --list: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		b := strings.TrimSpace(strings.TrimPrefix(line, "* "))
		if b == "" {
			continue
		}
		if exec.Command("git", "merge-base", "--is-ancestor", b, "dev").Run() == nil {
			continue // already merged
		}
		return b, nil
	}
	return "", fmt.Errorf("no unmerged %s* branch found", prefix)
}

// mtRebase checks out branch and rebases it onto dev.  On conflict it aborts
// the rebase, prints the conflicting paths, and returns an error.
func mtRebase(branch string) error {
	fmt.Printf("merge-train: rebasing %s onto dev...\n", branch)

	// Fetch origin if a remote exists (non-fatal).
	if remotes, err := exec.Command("git", "remote").Output(); err == nil && len(strings.TrimSpace(string(remotes))) > 0 {
		fetch := exec.Command("git", "fetch", "origin")
		fetch.Stdout = os.Stdout
		fetch.Stderr = os.Stderr
		_ = fetch.Run()
	}

	checkout := exec.Command("git", "checkout", branch)
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("git checkout %s: %w", branch, err)
	}

	rebase := exec.Command("git", "rebase", "dev")
	rebase.Stdout = os.Stdout
	rebase.Stderr = os.Stderr
	if err := rebase.Run(); err != nil {
		conflictOut, _ := exec.Command("git", "diff", "--name-only", "--diff-filter=U").Output()
		abort := exec.Command("git", "rebase", "--abort")
		abort.Stdout = os.Stdout
		abort.Stderr = os.Stderr
		_ = abort.Run()

		var b strings.Builder
		b.WriteString("forward-only — rebase conflicted on:\n")
		for _, f := range strings.Split(strings.TrimSpace(string(conflictOut)), "\n") {
			if f != "" {
				b.WriteString("  " + f + "\n")
			}
		}
		b.WriteString("resolve manually and re-run")
		return fmt.Errorf("%s", b.String())
	}
	return nil
}

// mtMigrationRenumber runs `dx migrate renumber --auto` to fix NNN collisions
// from concurrent branches.
func mtMigrationRenumber(root string) error {
	fmt.Println("merge-train: checking migration NNN collisions...")
	cmd := exec.Command(root+"/bin/dx", "migrate", "renumber", "--auto")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("migrate renumber: %w", err)
	}
	return nil
}

// mtHasNewMigrations reports whether the current HEAD (worker branch after
// rebase) introduces new migration up-files relative to dev.
func mtHasNewMigrations() bool {
	out, err := exec.Command("git", "log", "dev..HEAD", "--diff-filter=A",
		"--name-only", "--format=", "--", "internal/migrate/sql/*.up.sql").Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

// mtRegen runs all codegen steps in dependency order.
func mtRegen(root, branch string) error {
	// shipped.sql must be updated before sqlc generate when new migrations were added,
	// so that sqlc generates against the current schema.
	if mtHasNewMigrations() {
		fmt.Println("merge-train: new migrations detected — regenerating schema/shipped.sql...")
		regen := exec.Command(root + "/bin/regen-schema")
		regen.Dir = root
		regen.Stdout = os.Stdout
		regen.Stderr = os.Stderr
		if err := regen.Run(); err != nil {
			return fmt.Errorf("bin/regen-schema: %w", err)
		}
	}

	// sqlc generate — regenerates internal/db/ from schema/shipped.sql + queries/.
	fmt.Println("merge-train: running sqlc generate...")
	sqlcBin := mtSqlcPath()
	if err := cli.RunShell(sqlcBin+" generate", root); err != nil {
		return fmt.Errorf("sqlc generate: %w", err)
	}

	// dx ui gen-api — fetches spec from bin/dx-server --openapi, regenerates
	// ui/src/api.gen.ts (openapi-typescript) and internal/dxclient/models.gen.go
	// (oapi-codegen).  Subsumes `make gen-dxclient`.
	fmt.Println("merge-train: running dx ui gen-api...")
	if err := cli.RunShell(root+"/bin/dx ui gen-api", root); err != nil {
		return fmt.Errorf("dx ui gen-api: %w", err)
	}

	return nil
}

// mtCommitRegen stages all changes and creates a "regen: <branch>" commit if
// the working tree is dirty.  Dirty tree from a previous renumber step is
// folded into this commit.
func mtCommitRegen(branch string) error {
	out, err := exec.Command("git", "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("git status: %w", err)
	}
	if len(strings.TrimSpace(string(out))) == 0 {
		fmt.Println("merge-train: no regen needed")
		return nil
	}

	fmt.Println("merge-train: committing regen output...")
	add := exec.Command("git", "add", "-A")
	add.Stdout = os.Stdout
	add.Stderr = os.Stderr
	if err := add.Run(); err != nil {
		return fmt.Errorf("git add: %w", err)
	}

	commit := exec.Command("git", "commit", "-m", "regen: "+branch)
	commit.Stdout = os.Stdout
	commit.Stderr = os.Stderr
	if err := commit.Run(); err != nil {
		return fmt.Errorf("git commit: %w", err)
	}
	return nil
}

// mtFFMerge checks out dev and fast-forward merges branch into it.
func mtFFMerge(branch string) error {
	fmt.Println("merge-train: fast-forward merging into dev...")

	checkout := exec.Command("git", "checkout", "dev")
	checkout.Stdout = os.Stdout
	checkout.Stderr = os.Stderr
	if err := checkout.Run(); err != nil {
		return fmt.Errorf("git checkout dev: %w", err)
	}

	merge := exec.Command("git", "merge", "--ff-only", branch)
	merge.Stdout = os.Stdout
	merge.Stderr = os.Stderr
	if err := merge.Run(); err != nil {
		return fmt.Errorf("ff-merge failed (dev may have moved during run — re-run): %w", err)
	}
	return nil
}

func mtSqlcPath() string {
	if path, err := exec.LookPath("sqlc"); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	return home + "/go/bin/sqlc"
}

func mtGitOutput(args ...string) (string, error) {
	out, err := exec.Command(args[0], args[1:]...).Output()
	return strings.TrimSpace(string(out)), err
}
