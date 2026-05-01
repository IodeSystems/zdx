package ship

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/iodesystems/zdx-go/internal/config"
)

func TestRun_LocalSuccess(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "one", Run: "echo ok-one"},
		{Name: "two", Run: "echo ok-two"},
	}}}
	res, err := Run(context.Background(), comp, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	for i, want := range []string{"ok-one", "ok-two"} {
		if res[i].Status != "ok" {
			t.Errorf("stage %d: status=%q want ok", i, res[i].Status)
		}
		if !strings.Contains(res[i].Log, want) {
			t.Errorf("stage %d: log %q missing %q", i, res[i].Log, want)
		}
	}
}

func TestRun_EnvInjection(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Env: map[string]string{"FOO": "from-ship"},
		Stages: []config.Stage{
			{Name: "echo", Run: `echo $ZDX_DEPLOY_DIR-$FOO`},
		},
	}}
	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_DEPLOY_DIR": "/srv/app",
		"FOO":            "from-caller", // should be overridden by Ship.Env
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := strings.TrimSpace(res[0].Log); got != "/srv/app-from-ship" {
		t.Errorf("log = %q, want /srv/app-from-ship", got)
	}
}

func TestRun_OptionalFailureContinues(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "flaky", Run: "exit 1", Optional: true},
		{Name: "after", Run: "echo continued"},
	}}}
	res, err := Run(context.Background(), comp, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	if res[0].Status != "failed" || res[1].Status != "ok" {
		t.Errorf("statuses = [%q,%q], want [failed,ok]", res[0].Status, res[1].Status)
	}
}

func TestRun_NonOptionalHalts(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "halt", Run: "exit 1"},
		{Name: "never", Run: "echo unreached"},
	}}}
	res, err := Run(context.Background(), comp, nil)
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if res[0].Status != "failed" {
		t.Errorf("status = %q, want failed", res[0].Status)
	}
}

func TestBuildCmd_Local(t *testing.T) {
	stage := config.Stage{Name: "x", Run: "echo hi"}
	cmd := buildCmd(context.Background(), stage)
	want := []string{"sh", "-c", "echo hi"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildCmd_SSH(t *testing.T) {
	stage := config.Stage{Name: "x", Run: "uptime", Target: "deploy@host"}
	cmd := buildCmd(context.Background(), stage)
	want := []string{"ssh", "-o", "StrictHostKeyChecking=no", "-o", "BatchMode=yes", "deploy@host", "uptime"}
	if !reflect.DeepEqual(cmd.Args, want) {
		t.Errorf("args = %#v, want %#v", cmd.Args, want)
	}
}

func TestBuildCmd_SSH_PreservesRunString(t *testing.T) {
	run := "systemctl restart x && journalctl -u x -n 10"
	stage := config.Stage{Name: "x", Run: run, Target: "deploy@host"}
	cmd := buildCmd(context.Background(), stage)
	if got := cmd.Args[len(cmd.Args)-1]; got != run {
		t.Errorf("trailing argv = %q, want %q (no shell-quoting massaging)", got, run)
	}
}

// TestRun_DefaultsToSimple verifies that an empty Strategy and an
// explicit "simple" Strategy produce identical behavior (single pass,
// no extra env injected).
func TestRun_DefaultsToSimple(t *testing.T) {
	stages := []config.Stage{
		{Name: "one", Run: "echo a"},
		{Name: "two", Run: "echo b"},
	}
	empty := config.Component{Ship: config.Ship{Stages: stages}}
	explicit := config.Component{Ship: config.Ship{Strategy: config.ShipStrategySimple, Stages: stages}}

	resA, errA := Run(context.Background(), empty, nil)
	resB, errB := Run(context.Background(), explicit, nil)
	if errA != nil || errB != nil {
		t.Fatalf("Run errors: empty=%v explicit=%v", errA, errB)
	}
	if len(resA) != len(resB) {
		t.Fatalf("result counts differ: %d vs %d", len(resA), len(resB))
	}
	for i := range resA {
		if resA[i].Name != resB[i].Name || resA[i].Status != resB[i].Status {
			t.Errorf("result %d differs: %+v vs %+v", i, resA[i], resB[i])
		}
	}
}

// TestRun_BlueGreen verifies the two-pass blue-green flow: deploy pass
// runs every stage with caller's slot vars; verify pass re-runs only
// stages tagged "verify" with the slot vars swapped.
func TestRun_BlueGreen(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyBlueGreen,
		Stages: []config.Stage{
			{Name: "deploy", Run: "echo deploy:$ZDX_ACTIVE_SLOT"},
			{Name: "smoke", Run: "echo smoke:$ZDX_ACTIVE_SLOT", Tags: []string{"verify"}},
		},
	}}
	env := map[string]string{
		"ZDX_ACTIVE_SLOT":  "a",
		"ZDX_STANDBY_SLOT": "b",
	}
	res, err := Run(context.Background(), comp, env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 3 {
		t.Fatalf("want 3 results (2 deploy + 1 verify), got %d", len(res))
	}
	if got := strings.TrimSpace(res[0].Log); got != "deploy:a" {
		t.Errorf("pass1 deploy log = %q, want deploy:a", got)
	}
	if got := strings.TrimSpace(res[1].Log); got != "smoke:a" {
		t.Errorf("pass1 smoke log = %q, want smoke:a", got)
	}
	if res[2].Name != "smoke" {
		t.Errorf("pass2 stage name = %q, want smoke", res[2].Name)
	}
	if got := strings.TrimSpace(res[2].Log); got != "smoke:b" {
		t.Errorf("pass2 smoke log = %q, want smoke:b (swapped)", got)
	}
}

// TestRun_BlueGreen_NoVerifyStages verifies that blue-green with no
// verify-tagged stages skips the second pass cleanly.
func TestRun_BlueGreen_NoVerifyStages(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyBlueGreen,
		Stages: []config.Stage{
			{Name: "deploy", Run: "echo $ZDX_ACTIVE_SLOT-$ZDX_STANDBY_SLOT"},
		},
	}}
	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_ACTIVE_SLOT":  "a",
		"ZDX_STANDBY_SLOT": "b",
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result (no verify pass), got %d", len(res))
	}
	if got := strings.TrimSpace(res[0].Log); got != "a-b" {
		t.Errorf("log = %q, want a-b", got)
	}
}

// TestRun_BlueGreen_DeployFailureSkipsVerify verifies that a failure
// in the deploy pass aborts before the verify pass runs.
func TestRun_BlueGreen_DeployFailureSkipsVerify(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyBlueGreen,
		Stages: []config.Stage{
			{Name: "deploy", Run: "exit 1"},
			{Name: "smoke", Run: "echo never", Tags: []string{"verify"}},
		},
	}}
	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_ACTIVE_SLOT":  "a",
		"ZDX_STANDBY_SLOT": "b",
	})
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result (deploy failure halts), got %d", len(res))
	}
	if res[0].Status != "failed" {
		t.Errorf("status = %q, want failed", res[0].Status)
	}
}

// TestRun_Maintenance verifies that maintenance strategy injects ZDX_MAINTENANCE=1
// and that simple strategy does not.
func TestRun_Maintenance(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyMaintenance,
		Stages: []config.Stage{
			{Name: "check", Run: `echo ZDX_MAINTENANCE=$ZDX_MAINTENANCE`},
		},
	}}
	res, err := Run(context.Background(), comp, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result, got %d", len(res))
	}
	if !strings.Contains(res[0].Log, "ZDX_MAINTENANCE=1") {
		t.Errorf("log = %q, want ZDX_MAINTENANCE=1", res[0].Log)
	}

	// simple strategy must NOT inject ZDX_MAINTENANCE
	simple := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategySimple,
		Stages: []config.Stage{
			{Name: "check", Run: `echo ZDX_MAINTENANCE=$ZDX_MAINTENANCE`},
		},
	}}
	t.Setenv("ZDX_MAINTENANCE", "") // ensure it's absent from ambient env
	res2, err := Run(context.Background(), simple, nil)
	if err != nil {
		t.Fatalf("Run (simple): %v", err)
	}
	if strings.Contains(strings.TrimSpace(res2[0].Log), "ZDX_MAINTENANCE=1") {
		t.Errorf("simple strategy log = %q, must not contain ZDX_MAINTENANCE=1", res2[0].Log)
	}
}

// TestRun_RollingPair verifies the two-pass rolling-pair flow: pass 1
// runs every stage with ZDX_PHASE=current and ZDX_SLOT from ZDX_SLOT_A;
// pass 2 re-runs every stage with ZDX_PHASE=next and ZDX_SLOT from
// ZDX_SLOT_B.
func TestRun_RollingPair(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyRollingPair,
		Stages: []config.Stage{
			{Name: "deploy", Run: `echo $ZDX_PHASE-$ZDX_SLOT`},
		},
	}}
	env := map[string]string{
		"ZDX_SLOT_A": "a0",
		"ZDX_SLOT_B": "b1",
	}
	res, err := Run(context.Background(), comp, env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results (1 current + 1 next), got %d", len(res))
	}
	if got := strings.TrimSpace(res[0].Log); got != "current-a0" {
		t.Errorf("pass1 log = %q, want current-a0", got)
	}
	if got := strings.TrimSpace(res[1].Log); got != "next-b1" {
		t.Errorf("pass2 log = %q, want next-b1", got)
	}
}

// TestRun_RollingPair_FirstPassFailureSkipsSecond verifies that a
// non-optional failure in pass 1 halts before pass 2 runs.
func TestRun_RollingPair_FirstPassFailureSkipsSecond(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyRollingPair,
		Stages: []config.Stage{
			{Name: "deploy", Run: "exit 1"},
		},
	}}
	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_SLOT_A": "a0",
		"ZDX_SLOT_B": "b1",
	})
	if err == nil {
		t.Fatalf("want error, got nil")
	}
	if len(res) != 1 {
		t.Fatalf("want 1 result (first pass halts), got %d", len(res))
	}
	if res[0].Status != "failed" {
		t.Errorf("status = %q, want failed", res[0].Status)
	}
}

// TestRun_SSH_FakeSSH exercises the SSH path end-to-end with a fake `ssh`
// shim on PATH that just echoes its argv. Hermetic — gated only because
// it modifies $PATH for the test process.
func TestRun_SSH_FakeSSH(t *testing.T) {
	if os.Getenv("ZDX_SSH_INTEGRATION") == "" {
		t.Skip("set ZDX_SSH_INTEGRATION=1 to run")
	}
	dir := t.TempDir()
	shim := dir + "/ssh"
	if err := os.WriteFile(shim, []byte("#!/bin/sh\necho ssh-args: \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "remote", Run: "uptime", Target: "u@h"},
	}}}
	res, err := Run(context.Background(), comp, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res[0].Log, "ssh-args: -o StrictHostKeyChecking=no -o BatchMode=yes u@h uptime") {
		t.Errorf("log = %q does not contain expected ssh argv echo", res[0].Log)
	}
}
