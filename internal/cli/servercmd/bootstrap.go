package servercmd

import "github.com/iodesystems/zdx-go/internal/cli"

func bootstrapOnboardingIssue(c *cli.Client) error {
	return cli.BootstrapOnboardingIssue(c)
}
