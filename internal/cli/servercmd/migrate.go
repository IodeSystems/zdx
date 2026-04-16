package servercmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	"github.com/spf13/cobra"

	dxmigrate "github.com/iodesystems/zdx-go/internal/migrate"
)

func MigrateCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "migrate", Short: "Database migrations"}
	cmd.AddCommand(migrateUpCmd(), migrateVersionCmd())
	return cmd
}

func migrateUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Apply pending migrations",
		RunE: func(cmd *cobra.Command, args []string) error {
			dsn := requireDSN()
			if err := dxmigrate.Up(pgx5DSN(dsn)); err != nil {
				if errors.Is(err, migrate.ErrNoChange) {
					fmt.Println("no migrations to apply")
					return nil
				}
				return err
			}
			fmt.Println("✓ migrations applied")
			return nil
		},
	}
}

func migrateVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show current migration version",
		RunE: func(cmd *cobra.Command, args []string) error {
			v, dirty, err := dxmigrate.Version(pgx5DSN(requireDSN()))
			if err != nil {
				return err
			}
			if dirty {
				fmt.Printf("version: %d (dirty)\n", v)
			} else {
				fmt.Printf("version: %d\n", v)
			}
			return nil
		},
	}
}

func requireDSN() string {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_URL not set")
		os.Exit(1)
	}
	return dsn
}

func pgx5DSN(dsn string) string {
	return strings.Replace(dsn, "postgres://", "pgx5://", 1)
}

// RunMigrate kept for compatibility.
func RunMigrate(_ []string) {}
