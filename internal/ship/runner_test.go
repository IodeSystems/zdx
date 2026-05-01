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
	res, err := Run(context.Background(), comp, nil, RunOptions{})
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
	}, RunOptions{})
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
	res, err := Run(context.Background(), comp, nil, RunOptions{})
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
	res, err := Run(context.Background(), comp, nil, RunOptions{})
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

	resA, errA := Run(context.Background(), empty, nil, RunOptions{})
	resB, errB := Run(context.Background(), explicit, nil, RunOptions{})
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
	res, err := Run(context.Background(), comp, env, RunOptions{})
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
	}, RunOptions{})
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
	}, RunOptions{})
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
	res, err := Run(context.Background(), comp, nil, RunOptions{})
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
	res2, err := Run(context.Background(), simple, nil, RunOptions{})
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
	res, err := Run(context.Background(), comp, env, RunOptions{StateDir: t.TempDir()})
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
	}, RunOptions{StateDir: t.TempDir()})
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

// TestRun_RollingPair_Resume verifies that an interrupted rolling-pair
// re-run skips already-completed stages (the key acceptance criterion
// for IS-887). Simulated: write a state file with stage 1 completed,
// then run; expect stage 1 skipped and stage 2 executed.
func TestRun_RollingPair_Resume(t *testing.T) {
	dir := t.TempDir()
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyRollingPair,
		Stages: []config.Stage{
			{Name: "build", Run: "echo build-ran"},
			{Name: "deploy", Run: "echo deploy-ran"},
		},
	}}

	// Simulate: "build" already completed in the "current" pass.
	sf := stateFilePath(dir, gitSHA(), "mycomp", "current")
	if err := appendCompletedStage(sf, "build"); err != nil {
		t.Fatalf("setup state: %v", err)
	}

	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_SLOT_A": "a0",
		"ZDX_SLOT_B": "b1",
	}, RunOptions{StateDir: dir, ComponentName: "mycomp"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// current pass: build=skipped, deploy=ok
	// next pass:    build=ok,       deploy=ok
	if len(res) != 4 {
		t.Fatalf("want 4 results, got %d: %v", len(res), res)
	}
	if res[0].Name != "build" || res[0].Status != "skipped" {
		t.Errorf("res[0] = {%s %s}, want {build skipped}", res[0].Name, res[0].Status)
	}
	if res[1].Name != "deploy" || res[1].Status != "ok" {
		t.Errorf("res[1] = {%s %s}, want {deploy ok}", res[1].Name, res[1].Status)
	}
	if res[2].Name != "build" || res[2].Status != "ok" {
		t.Errorf("res[2] = {%s %s}, want {build ok} (next pass has no skip)", res[2].Name, res[2].Status)
	}
}

// TestRun_RollingPair_NoResume verifies that opts.NoResume deletes an
// existing state file so all stages execute from scratch.
func TestRun_RollingPair_NoResume(t *testing.T) {
	dir := t.TempDir()
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyRollingPair,
		Stages: []config.Stage{
			{Name: "build", Run: "echo build-ran"},
		},
	}}

	// Pre-populate state so that without --no-resume "build" would be skipped.
	sf := stateFilePath(dir, gitSHA(), "mycomp", "current")
	if err := appendCompletedStage(sf, "build"); err != nil {
		t.Fatalf("setup state: %v", err)
	}

	res, err := Run(context.Background(), comp, map[string]string{
		"ZDX_SLOT_A": "a0",
		"ZDX_SLOT_B": "b1",
	}, RunOptions{StateDir: dir, ComponentName: "mycomp", NoResume: true})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Both passes should execute build (not skipped).
	for i, r := range res {
		if r.Status != "ok" {
			t.Errorf("res[%d] status = %q, want ok (NoResume should force re-run)", i, r.Status)
		}
	}
}

// TestRun_FinalizeRunsAfterMainSuccess verifies that a finalize stage
// executes after all main stages succeed, and that its result is tagged
// Finalize=true.
func TestRun_FinalizeRunsAfterMainSuccess(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "main", Run: "echo main-ran"},
		{Name: "post", Run: "echo post-ran", Finalize: true},
	}}}
	res, err := Run(context.Background(), comp, nil, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	if res[0].Finalize || res[0].Status != "ok" {
		t.Errorf("res[0]: Finalize=%v Status=%q, want Finalize=false ok", res[0].Finalize, res[0].Status)
	}
	if !res[1].Finalize || res[1].Status != "ok" {
		t.Errorf("res[1]: Finalize=%v Status=%q, want Finalize=true ok", res[1].Finalize, res[1].Status)
	}
	if !strings.Contains(res[1].Log, "post-ran") {
		t.Errorf("finalize log = %q, want post-ran", res[1].Log)
	}
}

// TestRun_FinalizeFailureDoesNotFailHarness verifies that a failing finalize
// stage is recorded as "failed" but Run returns nil error.
func TestRun_FinalizeFailureDoesNotFailHarness(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "main", Run: "echo ok"},
		{Name: "post", Run: "exit 1", Finalize: true},
	}}}
	res, err := Run(context.Background(), comp, nil, RunOptions{})
	if err != nil {
		t.Fatalf("Run returned error, want nil: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results, got %d", len(res))
	}
	if res[1].Status != "failed" || !res[1].Finalize {
		t.Errorf("res[1]: Finalize=%v Status=%q, want Finalize=true failed", res[1].Finalize, res[1].Status)
	}
}

// TestRun_FinalizeSkippedWhenMainFails verifies that finalize stages are
// recorded as "skipped" (not run) when any main stage fails.
func TestRun_FinalizeSkippedWhenMainFails(t *testing.T) {
	comp := config.Component{Ship: config.Ship{Stages: []config.Stage{
		{Name: "main", Run: "exit 1"},
		{Name: "post", Run: "echo never", Finalize: true},
	}}}
	res, err := Run(context.Background(), comp, nil, RunOptions{})
	if err == nil {
		t.Fatal("want error from main failure, got nil")
	}
	if len(res) != 2 {
		t.Fatalf("want 2 results (main+finalize skipped), got %d", len(res))
	}
	if !res[1].Finalize || res[1].Status != "skipped" {
		t.Errorf("res[1]: Finalize=%v Status=%q, want Finalize=true skipped", res[1].Finalize, res[1].Status)
	}
}

// TestRun_RollingPair_FinalizeRunsOnce verifies that rolling-pair runs
// finalize stages exactly once after both passes — not per-pass.
func TestRun_RollingPair_FinalizeRunsOnce(t *testing.T) {
	comp := config.Component{Ship: config.Ship{
		Strategy: config.ShipStrategyRollingPair,
		Stages: []config.Stage{
			{Name: "deploy", Run: "echo $ZDX_PHASE"},
			{Name: "post", Run: "echo finalize-ran", Finalize: true},
		},
	}}
	env := map[string]string{"ZDX_SLOT_A": "a0", "ZDX_SLOT_B": "b1"}
	res, err := Run(context.Background(), comp, env, RunOptions{StateDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Expect: deploy(current), deploy(next), post(finalize x1)
	if len(res) != 3 {
		t.Fatalf("want 3 results (2 deploy + 1 finalize), got %d", len(res))
	}
	var finCount int
	for _, r := range res {
		if r.Finalize {
			finCount++
		}
	}
	if finCount != 1 {
		t.Errorf("finalize result count = %d, want 1", finCount)
	}
	if res[2].Name != "post" || res[2].Status != "ok" || !res[2].Finalize {
		t.Errorf("res[2] = {%s %s Finalize=%v}, want {post ok true}", res[2].Name, res[2].Status, res[2].Finalize)
	}
}

// TestRun_TagFilter verifies that IncludeTag and ExcludeTag in RunOptions
// correctly restrict which stages execute and appear in results.
func TestRun_TagFilter(t *testing.T) {
	stages := []config.Stage{
		{Name: "compile", Run: "echo compile", Tags: []string{"build"}},
		{Name: "tarball", Run: "echo tarball", Tags: []string{"build"}},
		{Name: "rsync", Run: "echo rsync", Tags: []string{"deploy"}},
		{Name: "restart", Run: "echo restart", Tags: []string{"deploy"}},
		{Name: "health", Run: "echo health"},
	}
	comp := config.Component{Ship: config.Ship{Stages: stages}}

	cases := []struct {
		name       string
		opts       RunOptions
		wantNames  []string
		wantCount  int
	}{
		{
			name:      "no filter runs all",
			opts:      RunOptions{},
			wantNames: []string{"compile", "tarball", "rsync", "restart", "health"},
			wantCount: 5,
		},
		{
			name:      "include=build runs only build-tagged",
			opts:      RunOptions{IncludeTag: "build"},
			wantNames: []string{"compile", "tarball"},
			wantCount: 2,
		},
		{
			name:      "exclude=build skips build-tagged",
			opts:      RunOptions{ExcludeTag: "build"},
			wantNames: []string{"rsync", "restart", "health"},
			wantCount: 3,
		},
		{
			name:      "include=deploy runs only deploy-tagged",
			opts:      RunOptions{IncludeTag: "deploy"},
			wantNames: []string{"rsync", "restart"},
			wantCount: 2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res, err := Run(context.Background(), comp, nil, tc.opts)
			if err != nil {
				t.Fatalf("Run: %v", err)
			}
			if len(res) != tc.wantCount {
				t.Fatalf("want %d results, got %d", tc.wantCount, len(res))
			}
			for i, name := range tc.wantNames {
				if res[i].Name != name {
					t.Errorf("res[%d].Name = %q, want %q", i, res[i].Name, name)
				}
				if res[i].Status != "ok" {
					t.Errorf("res[%d].Status = %q, want ok", i, res[i].Status)
				}
			}
		})
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
	res, err := Run(context.Background(), comp, nil, RunOptions{})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !strings.Contains(res[0].Log, "ssh-args: -o StrictHostKeyChecking=no -o BatchMode=yes u@h uptime") {
		t.Errorf("log = %q does not contain expected ssh argv echo", res[0].Log)
	}
}
