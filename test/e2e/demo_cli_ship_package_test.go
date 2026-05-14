package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDemoCLI_ShipPackageBinaryEmbedsSHA is a demo for spec 56:
// Given a clean git tree, when bin/ship --package runs, it builds the
// linux/amd64 binary with the current git SHA embedded via -X main.buildSHA,
// and writes a manifest.txt recording the full commit hash.
//
// Skipped when the working tree has uncommitted changes — ship itself would
// refuse, and the SHA assertion would be stale.
func TestDemoCLI_ShipPackageBinaryEmbedsSHA(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_package_test.go", Note: "spec 56 demo source"},
		{FilePath: "bin/ship", LineStart: 155, LineEnd: 210, Note: "build phase with -X main.buildSHA"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}

	// Skip if dirty — ship refuses dirty trees, and the SHA assertion would be stale.
	if out, _ := exec.Command("git", "-C", root, "status", "--porcelain").Output(); len(bytes.TrimSpace(out)) > 0 {
		t.Skipf("working tree has uncommitted changes; skipping --package integration test:\n%s", out)
	}

	shortSHAOut, err := exec.Command("git", "-C", root, "rev-parse", "--short", "HEAD").Output()
	if err != nil {
		t.Fatalf("git rev-parse --short: %v", err)
	}
	shortSHA := strings.TrimSpace(string(shortSHAOut))

	fullSHAOut, _ := exec.Command("git", "-C", root, "rev-parse", "HEAD").Output()
	fullSHA := strings.TrimSpace(string(fullSHAOut))

	cmd := exec.Command(filepath.Join(root, "bin", "ship"), "--package")
	cmd.Dir = root
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("bin/ship --package failed:\n%s", out.String())
	}

	binaryPath := filepath.Join(root, "deploy", "dist", "dx-server")
	binary, err := os.ReadFile(binaryPath)
	if err != nil {
		t.Fatalf("deploy/dist/dx-server not found after --package: %v", err)
	}
	if !bytes.Contains(binary, []byte(shortSHA)) {
		t.Errorf("deploy/dist/dx-server does not contain embedded build SHA %q", shortSHA)
	}

	manifestPath := filepath.Join(root, "deploy", "manifest.txt")
	manifest, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("deploy/manifest.txt not found: %v", err)
	}
	if !bytes.Contains(manifest, []byte(fullSHA)) {
		t.Errorf("deploy/manifest.txt missing commit %q:\n%s", fullSHA, manifest)
	}
}

// TestDemoCLI_ShipRollingDeployCallSequence is a demo for spec 56:
// Given a clean git tree, when bin/ship --no-package runs with all external
// commands replaced by record-only shims, it:
//  1. Runs the schema compatibility check (compat-check phase).
//  2. Performs a rolling deploy in the documented order:
//     rolling start → deploy next (rsync + migrate) → restart next →
//     rolling continue → deploy current (rsync) → restart current → rolling finalize.
//
// All infra calls (docker, go, rsync, ssh, infra/hz-client) are stubbed so no
// real host or container is touched.
func TestDemoCLI_ShipRollingDeployCallSequence(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_package_test.go", Note: "spec 56 demo source"},
		{FilePath: "bin/ship", LineStart: 309, LineEnd: 405, Note: "inline compat-check (run_compat_check)"},
		{FilePath: "bin/ship", LineStart: 488, LineEnd: 595, Note: "rolling deploy sequence"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}

	repo := newShipRepo(t, root)

	// ── Call log ──────────────────────────────────────────────────────────────
	callLog := filepath.Join(t.TempDir(), "calls.log")
	repo.env = append(repo.env, "CALL_LOG="+callLog)

	// ── PATH shims ────────────────────────────────────────────────────────────
	shimDir := t.TempDir()
	logPreamble := "CALL_LOG=\"${CALL_LOG:-/tmp/calls.log}\"\n"

	writeShim(t, shimDir, "docker", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"docker $*\" >> \"$CALL_LOG\"\n"+
		"for arg in \"$@\"; do\n"+
		"  [ \"$arg\" = \"pg_isready\" ] && exit 0\n"+
		"  if [ \"$arg\" = \"pg_dump\" ]; then echo \"-- stub schema\"; exit 0; fi\n"+
		"done\n"+
		"exit 0\n")
	writeShim(t, shimDir, "go", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"go $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "rsync", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"rsync $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "ssh", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"ssh $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "sqlc", "#!/usr/bin/env bash\nexit 0\n")

	// Prepend shim dir so it shadows any real docker/go/rsync/ssh/sqlc.
	for i, e := range repo.env {
		if strings.HasPrefix(e, "PATH=") {
			repo.env[i] = "PATH=" + shimDir + ":" + strings.TrimPrefix(e, "PATH=")
			break
		}
	}

	// ── Repo-relative stubs ───────────────────────────────────────────────────

	// infra/hz-client — outputs "Rolling phase: idle" for 'rolling status'.
	infraDir := filepath.Join(repo.root, "infra")
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	writeShim(t, infraDir, "hz-client", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"hz-client $*\" >> \"$CALL_LOG\"\n"+
		"prev=\"\"\n"+
		"for arg in \"$@\"; do\n"+
		"  if [ \"$prev\" = \"rolling\" ] && [ \"$arg\" = \"status\" ]; then\n"+
		"    echo \"Rolling phase: idle\"\n"+
		"  fi\n"+
		"  prev=\"$arg\"\n"+
		"done\n"+
		"exit 0\n")

	// deploy/dist/dx-server — just needs to exist for the --no-package guard.
	deployDistDir := filepath.Join(repo.root, "deploy", "dist")
	if err := os.MkdirAll(deployDistDir, 0o755); err != nil {
		t.Fatalf("mkdir deploy/dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deployDistDir, "dx-server"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("write dx-server stub: %v", err)
	}

	// deploy/dist/dx — invoked by run_compat_check for "dx migrate up".
	writeShim(t, deployDistDir, "dx", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"dx $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")

	// deploy/provision — rsync source for provisioning steps.
	if err := os.MkdirAll(filepath.Join(repo.root, "deploy", "provision"), 0o755); err != nil {
		t.Fatalf("mkdir deploy/provision: %v", err)
	}

	// schema/shipped.sql — empty → SKIP_CURRENT=true inside run_compat_check,
	// so only the NEXT-schema go test runs (both are stubbed).
	if err := os.MkdirAll(filepath.Join(repo.root, "schema"), 0o755); err != nil {
		t.Fatalf("mkdir schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.root, "schema", "shipped.sql"), nil, 0o644); err != nil {
		t.Fatalf("write shipped.sql: %v", err)
	}

	// internal/migrate/sql — empty dir prevents glob errors in shipped.sql validation.
	if err := os.MkdirAll(filepath.Join(repo.root, "internal", "migrate", "sql"), 0o755); err != nil {
		t.Fatalf("mkdir internal/migrate/sql: %v", err)
	}

	// home/deploy.secret.properties — provides host/token/hz_url without release_branch gate.
	homeDir := filepath.Join(repo.root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, "deploy.secret.properties"),
		[]byte("deploy.host=stub-host\ndeploy.token=stub-token\ndeploy.hz_url=http://localhost:19999\n"),
		0o644); err != nil {
		t.Fatalf("write deploy.secret.properties: %v", err)
	}

	// ── Run bin/ship --no-package ─────────────────────────────────────────────
	out, _ := repo.runShip(t, "--no-package")

	// (a) Compat-check banner confirms the compatibility gate ran.
	if !strings.Contains(out, "[ship] Running schema compatibility check") {
		t.Errorf("expected compat-check banner in ship output:\n%s", out)
	}

	// (b) Rolling-deploy call sequence matches the documented flow.
	callBytes, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("read call log %s: %v\nship output:\n%s", callLog, err, out)
	}
	calls := strings.Split(strings.TrimRight(string(callBytes), "\n"), "\n")

	assertCallOrder(t, calls, []string{
		"rolling start",
		"versions/next/",
		"bash -s",
		"restart zdx-next",
		"rolling continue",
		"versions/current/",
		"restart zdx-current",
		"rolling finalize",
	}, out)
}

// TestDemoCLI_ShipDeployRecordPost is a demo for spec 173 (must):
// Given a successful rolling deploy via bin/ship, when the deploy completes,
// then post_deploy_record() POSTs exactly once to the dx-server /deploys
// endpoint with build_sha, build_branch, status=success, duration_secs, log,
// and an Authorization: Bearer header — and the POST happens AFTER the
// 'rolling finalize' transition (so a failed finalize does not record a
// successful deploy).
//
// All infra calls are stubbed via PATH shims (curl included) so no real
// host or network is touched.
func TestDemoCLI_ShipDeployRecordPost(t *testing.T) {
	writeDemoCoderefs(t, t.Name(), []coderef{
		{FilePath: "test/e2e/demo_cli_ship_package_test.go", Note: "spec 173 demo source"},
		{FilePath: "bin/ship", LineStart: 268, LineEnd: 296, Note: "post_deploy_record() POST body + auth"},
		{FilePath: "bin/ship", LineStart: 658, LineEnd: 685, Note: "post_deploy_record called after rolling finalize"},
	})

	root, err := findRoot()
	if err != nil {
		t.Fatalf("findRoot: %v", err)
	}

	repo := newShipRepo(t, root)

	// ── Marker files for the curl shim ────────────────────────────────────────
	tmp := t.TempDir()
	callLog := filepath.Join(tmp, "calls.log")
	deployBody := filepath.Join(tmp, "deploy_body")
	deployAuth := filepath.Join(tmp, "deploy_auth")
	deployOrder := filepath.Join(tmp, "deploy_order")

	repo.env = append(repo.env,
		"CALL_LOG="+callLog,
		"DEPLOY_BODY_FILE="+deployBody,
		"DEPLOY_AUTH_FILE="+deployAuth,
		"DEPLOY_ORDER_FILE="+deployOrder,
	)

	// ── PATH shims ────────────────────────────────────────────────────────────
	shimDir := t.TempDir()
	logPreamble := "CALL_LOG=\"${CALL_LOG:-/tmp/calls.log}\"\n"

	writeShim(t, shimDir, "docker", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"docker $*\" >> \"$CALL_LOG\"\n"+
		"for arg in \"$@\"; do\n"+
		"  [ \"$arg\" = \"pg_isready\" ] && exit 0\n"+
		"  if [ \"$arg\" = \"pg_dump\" ]; then echo \"-- stub schema\"; exit 0; fi\n"+
		"done\n"+
		"exit 0\n")
	writeShim(t, shimDir, "go", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"go $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "rsync", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"rsync $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "ssh", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"ssh $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")
	writeShim(t, shimDir, "sqlc", "#!/usr/bin/env bash\nexit 0\n")

	// curl shim — captures POST body + auth header for /deploys, logs
	// every invocation to CALL_LOG so we can assert ordering vs 'rolling finalize'.
	// Captures either Authorization: or X-Api-Key: since bin/ship switched to
	// X-Api-Key in 89634386 to match the server's dual-auth scheme.
	writeShim(t, shimDir, "curl", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"curl $*\" >> \"$CALL_LOG\"\n"+
		"body=\"\"\n"+
		"auth=\"\"\n"+
		"url=\"\"\n"+
		"prev=\"\"\n"+
		"for arg in \"$@\"; do\n"+
		"  case \"$prev\" in\n"+
		"    -d|--data) body=\"$arg\" ;;\n"+
		"    -H)\n"+
		"      case \"$arg\" in Authorization:*|X-Api-Key:*) auth=\"$arg\" ;; esac\n"+
		"      ;;\n"+
		"  esac\n"+
		"  case \"$arg\" in http://*|https://*) url=\"$arg\" ;; esac\n"+
		"  prev=\"$arg\"\n"+
		"done\n"+
		"case \"$url\" in\n"+
		"  */deploys)\n"+
		"    printf '%s' \"$body\" > \"$DEPLOY_BODY_FILE\"\n"+
		"    printf '%s\\n' \"$auth\" > \"$DEPLOY_AUTH_FILE\"\n"+
		"    echo \"deploy-post\" >> \"$DEPLOY_ORDER_FILE\"\n"+
		"    echo '{\"id\":1}'\n"+
		"    ;;\n"+
		"esac\n"+
		"exit 0\n")

	// Prepend shim dir so it shadows real docker/go/rsync/ssh/sqlc/curl.
	for i, e := range repo.env {
		if strings.HasPrefix(e, "PATH=") {
			repo.env[i] = "PATH=" + shimDir + ":" + strings.TrimPrefix(e, "PATH=")
			break
		}
	}

	// ── Repo-relative stubs ───────────────────────────────────────────────────

	infraDir := filepath.Join(repo.root, "infra")
	if err := os.MkdirAll(infraDir, 0o755); err != nil {
		t.Fatalf("mkdir infra: %v", err)
	}
	writeShim(t, infraDir, "hz-client", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"hz-client $*\" >> \"$CALL_LOG\"\n"+
		"prev=\"\"\n"+
		"for arg in \"$@\"; do\n"+
		"  if [ \"$prev\" = \"rolling\" ] && [ \"$arg\" = \"status\" ]; then\n"+
		"    echo \"Rolling phase: idle\"\n"+
		"  fi\n"+
		"  prev=\"$arg\"\n"+
		"done\n"+
		"exit 0\n")

	deployDistDir := filepath.Join(repo.root, "deploy", "dist")
	if err := os.MkdirAll(deployDistDir, 0o755); err != nil {
		t.Fatalf("mkdir deploy/dist: %v", err)
	}
	if err := os.WriteFile(filepath.Join(deployDistDir, "dx-server"), []byte("stub"), 0o644); err != nil {
		t.Fatalf("write dx-server stub: %v", err)
	}
	writeShim(t, deployDistDir, "dx", "#!/usr/bin/env bash\n"+logPreamble+
		"echo \"dx $*\" >> \"$CALL_LOG\"\n"+
		"exit 0\n")

	if err := os.MkdirAll(filepath.Join(repo.root, "deploy", "provision"), 0o755); err != nil {
		t.Fatalf("mkdir deploy/provision: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(repo.root, "schema"), 0o755); err != nil {
		t.Fatalf("mkdir schema: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo.root, "schema", "shipped.sql"), nil, 0o644); err != nil {
		t.Fatalf("write shipped.sql: %v", err)
	}

	if err := os.MkdirAll(filepath.Join(repo.root, "internal", "migrate", "sql"), 0o755); err != nil {
		t.Fatalf("mkdir internal/migrate/sql: %v", err)
	}

	// home/deploy.secret.properties — includes the four new keys (IS-667) so
	// post_deploy_record fires instead of warn-and-skipping.
	homeDir := filepath.Join(repo.root, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatalf("mkdir home: %v", err)
	}
	props := strings.Join([]string{
		"deploy.host=stub-host",
		"deploy.token=stub-token",
		"deploy.hz_url=http://localhost:19999",
		"deploy.project_slug=stub-project",
		"deploy.environment_name=stub-env",
		"deploy.api_url=http://stub-api",
		"deploy.api_token=stub-api-token",
		"",
	}, "\n")
	if err := os.WriteFile(filepath.Join(homeDir, "deploy.secret.properties"), []byte(props), 0o644); err != nil {
		t.Fatalf("write deploy.secret.properties: %v", err)
	}

	// ── Run bin/ship --no-package ─────────────────────────────────────────────
	out, _ := repo.runShip(t, "--no-package")

	// (a) POST happened exactly once.
	orderBytes, err := os.ReadFile(deployOrder)
	if err != nil {
		t.Fatalf("read deploy order file %s: %v\nship output:\n%s", deployOrder, err, out)
	}
	orderLines := strings.Split(strings.TrimRight(string(orderBytes), "\n"), "\n")
	if len(orderLines) != 1 || orderLines[0] != "deploy-post" {
		t.Errorf("expected exactly one POST to /deploys; got %d:\n%s\nship output:\n%s",
			len(orderLines), string(orderBytes), out)
	}

	// (b) JSON body has required fields with sane values.
	bodyBytes, err := os.ReadFile(deployBody)
	if err != nil {
		t.Fatalf("read deploy body file: %v\nship output:\n%s", err, out)
	}
	var body map[string]any
	if err := json.Unmarshal(bodyBytes, &body); err != nil {
		t.Fatalf("deploy body is not valid JSON: %v\nbody: %s", err, bodyBytes)
	}
	if s, _ := body["build_sha"].(string); s == "" {
		t.Errorf("build_sha is empty in body: %s", bodyBytes)
	}
	if s, _ := body["build_branch"].(string); s == "" {
		t.Errorf("build_branch is empty in body: %s", bodyBytes)
	}
	if s, _ := body["status"].(string); s != "success" {
		t.Errorf("status != \"success\" in body: %s", bodyBytes)
	}
	// jq emits numeric values as float64 in Go's JSON decoder.
	if d, _ := body["duration_secs"].(float64); d < 0 {
		t.Errorf("duration_secs < 0 in body: %s", bodyBytes)
	} else if _, ok := body["duration_secs"]; !ok {
		t.Errorf("duration_secs missing from body: %s", bodyBytes)
	}
	if _, ok := body["log"]; !ok {
		t.Errorf("log missing from body: %s", bodyBytes)
	}

	// (c) X-Api-Key header present with the configured token. bin/ship was
	// changed in 89634386 from `Authorization: Bearer <token>` to
	// `X-Api-Key: <token>` so the deploy-record POST matches the dual-auth
	// scheme the server already supports (X-Api-Key is the primary, Bearer
	// the legacy fallback). This assertion mirrors that change.
	authBytes, err := os.ReadFile(deployAuth)
	if err != nil {
		t.Fatalf("read deploy auth file: %v\nship output:\n%s", err, out)
	}
	auth := strings.TrimSpace(string(authBytes))
	if !strings.HasPrefix(auth, "X-Api-Key: ") {
		t.Errorf("expected X-Api-Key header; got %q", auth)
	}
	if !strings.Contains(auth, "stub-api-token") {
		t.Errorf("expected configured api token in auth header; got %q", auth)
	}

	// (d) curl POST to /deploys appears AFTER 'rolling finalize' in the call log.
	callBytes, err := os.ReadFile(callLog)
	if err != nil {
		t.Fatalf("read call log %s: %v\nship output:\n%s", callLog, err, out)
	}
	calls := strings.Split(strings.TrimRight(string(callBytes), "\n"), "\n")
	assertCallOrder(t, calls, []string{
		"rolling finalize",
		"/deploys",
	}, out)
}

// writeShim writes an executable shell script at dir/name.
func writeShim(t *testing.T, dir, name, content string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o755); err != nil {
		t.Fatalf("write shim %s: %v", p, err)
	}
}

// assertCallOrder verifies each want substring appears in order within calls.
// shipOutput is included in failure messages for context.
func assertCallOrder(t *testing.T, calls, want []string, shipOutput string) {
	t.Helper()
	j := 0
	for _, call := range calls {
		if j >= len(want) {
			break
		}
		if strings.Contains(call, want[j]) {
			j++
		}
	}
	if j < len(want) {
		t.Errorf("call sequence stopped at step %d: missing %q\ncall log:\n%s\nship output:\n%s",
			j, want[j], strings.Join(calls, "\n"), shipOutput)
	}
}
