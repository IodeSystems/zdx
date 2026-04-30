package devtools

import (
	"github.com/spf13/cobra"
)

func ShipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Ship-time checks and helpers",
	}
	cmd.AddCommand(shipGateCmd())
	return cmd
}

func shipGateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "gate",
		Short: "Check must-tier spec demos before deploy",
		Long:  "Runs must-tier demo tests. Exits non-zero if any fail — bin/ship uses this to block deploy.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			testCmd := TestCmd()
			if err := testCmd.ParseFlags([]string{
				"--importance=must",
				"--layer=demo",
				"--no-ephemeral",
			}); err != nil {
				return err
			}
			return testCmd.RunE(testCmd, nil)
		},
	}
}
