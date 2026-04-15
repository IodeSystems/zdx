package e2e

import (
	"os"
	"strings"
	"testing"
)

// DriverMode controls which test driver(s) run.
type DriverMode int

const (
	DriverAPI DriverMode = iota
	DriverUI
	DriverAll
)

func (d DriverMode) String() string {
	switch d {
	case DriverAPI:
		return "api"
	case DriverUI:
		return "ui"
	case DriverAll:
		return "all"
	default:
		return "unknown"
	}
}

var activeDriver = DriverAPI

func initDriverMode() {
	switch strings.ToLower(os.Getenv("TEST_DRIVER")) {
	case "ui":
		activeDriver = DriverUI
	case "all":
		activeDriver = DriverAll
	default:
		activeDriver = DriverAPI
	}
}

// skipUnlessDriver skips the test if the current driver mode doesn't include the requested mode.
func skipUnlessDriver(t *testing.T, mode DriverMode) {
	t.Helper()
	if activeDriver == DriverAll {
		return
	}
	if activeDriver != mode {
		t.Skipf("skipping: requires %s driver, active is %s", mode, activeDriver)
	}
}

// requiresUI skips the test unless UI driver is active.
func requiresUI(t *testing.T) {
	t.Helper()
	skipUnlessDriver(t, DriverUI)
}

// requiresAPI skips the test unless API driver is active.
func requiresAPI(t *testing.T) {
	t.Helper()
	skipUnlessDriver(t, DriverAPI)
}

// runOnEachDriver runs a subtest for each active driver mode.
func runOnEachDriver(t *testing.T, name string, fn func(t *testing.T, mode DriverMode)) {
	t.Helper()
	modes := []DriverMode{DriverAPI}
	if activeDriver == DriverUI {
		modes = []DriverMode{DriverUI}
	} else if activeDriver == DriverAll {
		modes = []DriverMode{DriverAPI, DriverUI}
	}
	for _, m := range modes {
		t.Run(name+"/"+m.String(), func(t *testing.T) {
			fn(t, m)
		})
	}
}
