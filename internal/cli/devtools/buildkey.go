package devtools

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/iodesystems/zdx-go/internal/deploypkg"
)

// defaultBuildKeyPath is the on-disk default for the Ed25519 build-key.
// Gitignored. $ZDX_BUILD_KEY_PATH overrides it; --path overrides both.
const defaultBuildKeyPath = ".zdx/build-key.pem"

func resolveBuildKeyPath(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("ZDX_BUILD_KEY_PATH"); v != "" {
		return v
	}
	return defaultBuildKeyPath
}

func shipBuildKeyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-key",
		Short: "Manage the Ed25519 build-key used to sign deploy packages",
	}
	cmd.AddCommand(shipBuildKeyInitCmd())
	cmd.AddCommand(shipBuildKeyFingerprintCmd())
	return cmd
}

func shipBuildKeyInitCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Generate a new build-key, save to disk, print fingerprint",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := resolveBuildKeyPath(path)
			priv, pub, err := deploypkg.GenerateBuildKey()
			if err != nil {
				return err
			}
			if err := deploypkg.SaveBuildKey(p, priv); err != nil {
				return err
			}
			fmt.Printf("wrote %s (mode 0600)\n", p)
			fmt.Printf("fingerprint: %s\n", deploypkg.Fingerprint(pub))
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "build-key path (default: $ZDX_BUILD_KEY_PATH or "+defaultBuildKeyPath+")")
	return cmd
}

func shipBuildKeyFingerprintCmd() *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:          "fingerprint",
		Short:        "Print the fingerprint of an existing build-key",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p := resolveBuildKeyPath(path)
			priv, err := deploypkg.LoadBuildKey(p)
			if err != nil {
				return err
			}
			fp, err := deploypkg.FingerprintPriv(priv)
			if err != nil {
				return err
			}
			fmt.Println(fp)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "path", "", "build-key path (default: $ZDX_BUILD_KEY_PATH or "+defaultBuildKeyPath+")")
	return cmd
}
