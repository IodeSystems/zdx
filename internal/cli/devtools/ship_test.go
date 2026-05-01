package devtools

import "testing"

func TestShipCmdHasRunSubcommand(t *testing.T) {
	cmd := ShipCmd()
	run, _, err := cmd.Find([]string{"run"})
	if err != nil {
		t.Fatalf("Find(run): %v", err)
	}
	if run == nil || run.Use != "run" {
		t.Fatal("ship run subcommand not registered")
	}
	if cmd := run.Flags().Lookup("env"); cmd == nil {
		t.Error("--env flag missing")
	}
	if cmd := run.Flags().Lookup("component"); cmd == nil {
		t.Error("--component flag missing")
	}
	if cmd := run.Flags().Lookup("non-compatible-migration"); cmd == nil {
		t.Error("--non-compatible-migration flag missing")
	}
}
