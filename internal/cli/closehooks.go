package cli

import (
	"fmt"

	"github.com/iodesystems/zdx-go/internal/config"
)

// RunCloseHooks executes all close hooks defined in .zdx/config.yaml.
// Called when an issue or task is closed via the CLI.
func RunCloseHooks() error {
	cfg := config.Load()
	if cfg == nil {
		return nil
	}
	steps := cfg.AllCloseSteps("")
	if len(steps) == 0 {
		return nil
	}
	for _, ns := range steps {
		label := ns.Name
		if label == "" {
			label = ns.Run
		}
		fmt.Printf("[close] %s: %s\n", ns.Component, label)
		if err := RunShell(ns.Run, ns.CWD); err != nil {
			return fmt.Errorf("close hook %q failed: %w", label, err)
		}
	}
	return nil
}
