package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/cli"
)

func main() {
	root := &cobra.Command{
		Use:           "dx",
		Short:         "Developer experience CLI",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		cli.TodoCmd(),
		cli.IssueCmd(),
		cli.BuildCmd(),
		cli.TestCmd(),
		cli.LintCmd(),
		cli.CheckCmd(),
		cli.WatchCmd(),
		cli.HooksCmd(),
		cli.CtxCmd(),
		cli.InitCmd(),
		cli.DaemonCmd(),
		cli.MigrateCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
