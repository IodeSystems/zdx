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
