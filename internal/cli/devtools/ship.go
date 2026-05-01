package devtools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/config"
	"github.com/iodesystems/zdx-go/internal/dxclient"
	"github.com/iodesystems/zdx-go/internal/migrate"
	"github.com/iodesystems/zdx-go/internal/ship"
	"github.com/iodesystems/zdx-go/internal/ship/preflight"
)

func ShipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship-time checks and helpers",
	}
	cmd.AddCommand(shipGateCmd())
	cmd.AddCommand(shipRunCmd())
	cmd.AddCommand(shipCompatCheckCmd())
	cmd.AddCommand(shipMigratePreflightCmd())
	return cmd
}

func shipMigratePreflightCmd() *cobra.Command {
	var deployedSHA, migDir, fixesDir, sshHost, remoteHome, envName string
	cmd := &cobra.Command{
		Use:   "migrate-preflight",
		Short: "Validate shipped migration history against the deployed commit",
		Long: `Replays bin/ship's migration history check in Go.

Resolves the deployed commit SHA in this order: --deployed-sha, dx-server
environment record (--env), or 'ssh <host> cat <remote-home>/home/shipped-commit'.
For each migration in the deployed commit's tree, refuses to ship if the local
copy diverges without a matching internal/migrate/sql/migration-fixes/<base>-<server8>-<dev8>.sql
file. Pure renames are allowed; first deploy is a no-op.

Pending fix files (acknowledged divergences) are printed under a PENDING_FIXES:
prefix on stdout so callers (the ship pipeline, packaging stages) can rsync
them to the host.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			sha := deployedSHA
			if sha == "" && envName != "" {
				resolved, err := resolveDeployedSHAFromAPI(ctx, envName)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[ship] WARN: API lookup of deployed SHA failed: %v\n", err)
				} else {
					sha = resolved
				}
			}
			if sha == "" && sshHost != "" {
				home := remoteHome
				if home == "" {
					home = "/home/zdx"
				}
				out, err := exec.CommandContext(ctx, "ssh", sshHost,
					"cat "+home+"/home/shipped-commit 2>/dev/null").Output()
				if err == nil {
					sha = strings.TrimSpace(string(out))
				}
			}

			fmt.Println("[ship] Validating migration history...")
			notes, fixes, runErr := preflight.MigrationHistory(ctx, preflight.Options{
				DeployedSHA:  sha,
				MigrationDir: migDir,
				FixesDir:     fixesDir,
			})

			for _, n := range notes {
				if n.Kind == preflight.NoteFirstDeploy {
					fmt.Println("[ship] No deployed commit — first deploy, skipping history check.")
					continue
				}
				fmt.Printf("[ship] NOTE: %s\n", n.Msg)
			}

			if runErr != nil {
				for _, line := range strings.Split(runErr.Error(), "\n") {
					if strings.HasPrefix(line, "  ") {
						fmt.Printf("[ship] %s\n", line)
					} else {
						fmt.Printf("[ship] ERROR: %s\n", line)
					}
				}
				fmt.Println("[ship] REFUSING: shipped migrations modified — create a new migration instead.")
				return runErr
			}

			if len(fixes) > 0 {
				fmt.Println("PENDING_FIXES:")
				for _, f := range fixes {
					fmt.Println(f.Path)
				}
			}

			// FIRST_DEPLOY note already printed above; suppress the OK line in that case.
			if !(len(notes) == 1 && notes[0].Kind == preflight.NoteFirstDeploy) {
				fmt.Println("[ship] Migration history OK.")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&deployedSHA, "deployed-sha", "", "deployed commit SHA (skips API+ssh resolution)")
	cmd.Flags().StringVar(&migDir, "migration-dir", "internal/migrate/sql", "directory holding *.sql migrations")
	cmd.Flags().StringVar(&fixesDir, "fixes-dir", "", "directory holding migration-fixes/ files (default: <migration-dir>/migration-fixes)")
	cmd.Flags().StringVar(&sshHost, "ssh-host", "", "ssh host for shipped-commit fallback (e.g. user@host)")
	cmd.Flags().StringVar(&remoteHome, "remote-home", "", "remote home dir for shipped-commit (default: /home/zdx)")
	cmd.Flags().StringVar(&envName, "env", "", "environment name to query for current_build_sha via dx-server API")
	return cmd
}

// resolveDeployedSHAFromAPI hits GET /api/dx/projects/{slug}/environments/{env}
// and returns CurrentBuildSha. Empty SHA → empty string + nil error (legitimate
// first-deploy state).
func resolveDeployedSHAFromAPI(ctx context.Context, env string) (string, error) {
	c, err := cli.DefaultClient()
	if err != nil {
		return "", err
	}
	slug := c.SlugOrDie()
	resp, err := c.GetEnvironmentWithResponse(ctx, slug, env)
	if err != nil {
		return "", err
	}
	if resp.JSON200 == nil {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode())
	}
	return resp.JSON200.CurrentBuildSha, nil
}

func shipGitOutput(args ...string) string {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func shipRunCmd() *cobra.Command {
	var envFlag, componentFlag string
	var noResume bool
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute ship stages for a component and record the deploy event",
		RunE: func(cmd *cobra.Command, _ []string) error {
			compName := config.ActiveComponent(componentFlag)
			cfg := config.Load()
			if cfg == nil {
				return fmt.Errorf("no .zdx/config.yaml found")
			}
			comp, ok := cfg.Components[compName]
			if !ok {
				return fmt.Errorf("component %q not found", compName)
			}
			if comp.Ship.IsZero() {
				return fmt.Errorf("component %q has no ship config", compName)
			}

			sha := shipGitOutput("rev-parse", "HEAD")
			branch := shipGitOutput("rev-parse", "--abbrev-ref", "HEAD")

			opts := ship.RunOptions{
				NoResume:      noResume,
				ComponentName: compName,
			}
			results, runErr := ship.Run(cmd.Context(), comp, nil, opts)

			c, err := cli.DefaultClient()
			if err != nil {
				return err
			}
			slug := c.SlugOrDie()
			_ = ship.PostDeployEvent(cmd.Context(), c, slug, envFlag, sha, branch, results)
			ship.ReportFinalizeFailures(cmd.Context(), c, slug, comp.Ship.Strategy, results)

			return runErr
		},
	}
	cmd.Flags().StringVar(&envFlag, "env", "", "environment name to record the deploy against (required)")
	_ = cmd.MarkFlagRequired("env")
	cmd.Flags().StringVar(&componentFlag, "component", "", "component name (default: active component from config or DX_COMPONENT)")
	cmd.Flags().BoolVar(&noResume, "no-resume", false, "force full re-run, ignoring saved stage state")
	return cmd
}

func shipGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gate",
		Short: "Check must-tier spec demos before deploy",
		Long:  "Queries must-tier spec demo gaps via the API. Exits non-zero if any must-specs lack passing demos. Informational should/nice-to-have counts are printed but never affect exit code.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := cli.DefaultClient()
			if err != nil {
				return fmt.Errorf("ship gate: %w", err)
			}
			slug := c.SlugOrDie()
			ctx := cmd.Context()

			// ── Must-spec gate (blocking) ──────────────────────────────────────
			mustResp, err := c.ListMustSpecShipGateOffendersWithResponse(ctx, &dxclient.ListMustSpecShipGateOffendersParams{
				Slug: slug,
			})
			if err != nil {
				return fmt.Errorf("ship gate: could not fetch must-spec demo gaps: %v", err)
			}
			if mustResp.JSON200 == nil {
				if mustResp.StatusCode() == 404 {
					fmt.Println("[ship gate] endpoint not yet deployed — skipping must-spec check")
					return nil
				}
				return fmt.Errorf("ship gate: could not fetch must-spec demo gaps: HTTP %d", mustResp.StatusCode())
			}
			offenders := *mustResp.JSON200.Offenders

			// ── Informational should/nice-to-have counts ───────────────────────
			gapResp, gapErr := c.ListSpecsWithoutDemosWithResponse(ctx, &dxclient.ListSpecsWithoutDemosParams{
				Slug: slug,
			})
			var shouldCount, niceCount int
			if gapErr == nil && gapResp.JSON200 != nil {
				for _, s := range *gapResp.JSON200.Specs {
					switch s.Importance {
					case "should":
						shouldCount++
					case "nice-to-have":
						niceCount++
					}
				}
			}

			if len(offenders) == 0 {
				fmt.Println("[ship gate] No must-spec demo gaps. Ready to ship.")
				if shouldCount > 0 || niceCount > 0 {
					fmt.Printf("[ship gate] (informational) shoulds: %d, nice-to-have: %d\n", shouldCount, niceCount)
				}
				return nil
			}

			var b strings.Builder
			fmt.Fprintf(&b, "[ship gate] Must-spec demo gaps — deploy blocked:\n")
			for _, o := range offenders {
				fmt.Fprintf(&b, "  SP-%d  %s  %s  (%s)\n", o.SpecId, o.Feature, o.Description, o.Reason)
			}
			if shouldCount > 0 || niceCount > 0 {
				fmt.Fprintf(&b, "[ship gate] (informational) shoulds: %d, nice-to-have: %d\n", shouldCount, niceCount)
			}
			fmt.Fprint(os.Stderr, b.String())
			return fmt.Errorf("must-spec gate: %d offender(s)", len(offenders))
		},
	}
}

const (
	compatPgImage = "postgres:17"
	compatPgUser  = "zdx"
	compatPgPass  = "zdx"
	compatPgDB    = "zdx"
	compatPortCur = "17650"
	compatPortNxt = "17651"
	compatNameCur = "zdx-compat-current"
	compatNameNxt = "zdx-compat-next"
)

func shipCompatCheckCmd() *cobra.Command {
	var dsnCurrent, dsnNext string
	cmd := &cobra.Command{
		Use:   "compat-check",
		Short: "Schema compatibility check: ephemeral Postgres + migration + tests",
		Long: `Spins up two ephemeral Postgres containers (or uses --dsn-current/--dsn-next for CI),
applies shipped.sql to current, runs all migrations on next, pg_dumps next schema,
runs 'go test ./...' against both DSNs, then writes schema/compat-result.txt.

Exit codes: 0=compatible, 1=incompatible migration, 2=broken tests or infra failure.`,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			useDocker := dsnCurrent == "" && dsnNext == ""
			if useDocker {
				dsnCurrent = fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable",
					compatPgUser, compatPgPass, compatPortCur, compatPgDB)
				dsnNext = fmt.Sprintf("postgres://%s:%s@localhost:%s/%s?sslmode=disable",
					compatPgUser, compatPgPass, compatPortNxt, compatPgDB)

				fmt.Println("[compat] Starting ephemeral Postgres containers...")
				if err := compatStartPg(compatNameCur, compatPortCur); err != nil {
					return fmt.Errorf("compat: %w", err)
				}
				if err := compatStartPg(compatNameNxt, compatPortNxt); err != nil {
					compatRemove(compatNameCur)
					return fmt.Errorf("compat: %w", err)
				}
				defer func() {
					compatRemove(compatNameCur)
					compatRemove(compatNameNxt)
				}()

				fmt.Println("[compat] Waiting for Postgres...")
				if err := compatWaitPg(compatNameCur); err != nil {
					return err
				}
				if err := compatWaitPg(compatNameNxt); err != nil {
					return err
				}
			}

			// Apply shipped schema to current container (skip if absent/empty).
			skipCurrent := false
			shippedSQL, err := os.ReadFile("schema/shipped.sql")
			if err != nil || len(bytes.TrimSpace(shippedSQL)) == 0 {
				fmt.Println("[compat] No shipped schema yet — skipping current-schema test.")
				skipCurrent = true
			} else {
				fmt.Println("[compat] Applying shipped schema to current container...")
				if err := compatPsql(compatNameCur, shippedSQL); err != nil {
					return fmt.Errorf("compat: apply shipped.sql: %w", err)
				}
			}

			// Apply all migrations to next container.
			fmt.Println("[compat] Applying migrations to next container...")
			if err := migrate.Up(dsnNext); err != nil {
				return fmt.Errorf("compat: migrate up: %w", err)
			}

			// pg_dump next schema → schema/next.sql
			fmt.Println("[compat] Dumping next schema...")
			if err := compatDumpSchema(compatNameNxt, "schema/next.sql"); err != nil {
				return fmt.Errorf("compat: pg_dump: %w", err)
			}

			// Run tests against next schema.
			fmt.Println("[compat] Running tests against NEXT schema...")
			pkgs, err := compatListPackages()
			if err != nil {
				return fmt.Errorf("compat: list packages: %w", err)
			}
			if err := compatRunTests(dsnNext, pkgs); err != nil {
				fmt.Println("[compat] FAIL: tests broken against next schema.")
				_ = os.WriteFile("schema/compat-result.txt", []byte("incompatible"), 0o644)
				fmt.Fprintln(os.Stderr, "compat-check: tests failed on next schema")
				os.Exit(2)
			}
			fmt.Println("[compat] NEXT schema: OK")

			if skipCurrent {
				fmt.Println("[compat] No current schema to check — assuming compatible.")
				_ = os.WriteFile("schema/compat-result.txt", []byte("compatible"), 0o644)
				return nil
			}

			// Run tests against current schema (rolling-deploy gate).
			fmt.Println("[compat] Running tests against CURRENT schema (rolling-deploy gate)...")
			if err := compatRunTests(dsnCurrent, pkgs); err != nil {
				fmt.Println("[compat] CURRENT schema: FAIL — migration is INCOMPATIBLE.")
				_ = os.WriteFile("schema/compat-result.txt", []byte("incompatible"), 0o644)
				// Exit 1 = incompatible (not infra/test failure).
				return fmt.Errorf("compat-check: incompatible migration")
			}
			fmt.Println("[compat] CURRENT schema: OK — migration is COMPATIBLE.")
			_ = os.WriteFile("schema/compat-result.txt", []byte("compatible"), 0o644)
			return nil
		},
	}
	cmd.Flags().StringVar(&dsnCurrent, "dsn-current", "", "DSN for current-schema DB (CI: skip Docker)")
	cmd.Flags().StringVar(&dsnNext, "dsn-next", "", "DSN for next-schema DB (CI: skip Docker)")
	return cmd
}

func compatStartPg(name, port string) error {
	return exec.Command("docker", "run", "-d", "--name", name,
		"-e", "POSTGRES_USER="+compatPgUser,
		"-e", "POSTGRES_PASSWORD="+compatPgPass,
		"-e", "POSTGRES_DB="+compatPgDB,
		"-p", "127.0.0.1:"+port+":5432",
		compatPgImage,
	).Run()
}

func compatRemove(name string) {
	_ = exec.Command("docker", "rm", "-f", name).Run()
}

func compatWaitPg(name string) error {
	for i := 0; i < 30; i++ {
		if exec.Command("docker", "exec", name, "pg_isready", "-U", compatPgUser, "-q").Run() == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return fmt.Errorf("compat: %s did not become ready", name)
}

func compatPsql(containerName string, sql []byte) error {
	cmd := exec.Command("docker", "exec", "-i", containerName, "psql", "-U", compatPgUser, "-d", compatPgDB)
	cmd.Stdin = bytes.NewReader(sql)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func compatDumpSchema(containerName, dest string) error {
	out, err := exec.Command("docker", "exec", containerName,
		"pg_dump", "-U", compatPgUser, "-d", compatPgDB,
		"--schema-only", "--no-owner", "--no-privileges",
		"--exclude-table=schema_migrations",
	).Output()
	if err != nil {
		return err
	}

	// Filter noise lines (backslash commands and the pg_dump version comment).
	var filtered []string
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "\\") || strings.HasPrefix(line, "-- Dumped by pg_dump version") {
			continue
		}
		filtered = append(filtered, line)
	}
	return os.WriteFile(dest, []byte(strings.Join(filtered, "\n")), 0o644)
}

func compatListPackages() ([]string, error) {
	out, err := exec.Command("go", "list", "./...").Output()
	if err != nil {
		return nil, err
	}
	var pkgs []string
	for _, p := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if !strings.Contains(p, "/test/e2e") {
			pkgs = append(pkgs, p)
		}
	}
	return pkgs, nil
}

func compatRunTests(dsn string, pkgs []string) error {
	args := append([]string{"test"}, pkgs...)
	args = append(args, "-count=1", "-timeout=60s")
	cmd := exec.Command("go", args...)
	cmd.Env = append(os.Environ(), "DATABASE_URL="+dsn)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
