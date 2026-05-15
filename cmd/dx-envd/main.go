// dx-envd is the per-environment daemon (IS-1227 / IS-1233). One copy
// runs on each deploy host, holds a persistent WS to dx-server, and is
// the only actor with deploy privileges on that env. This entry point is
// the binary scaffold + heartbeat path; deploy/schema-dump command
// handlers are added by IS-1229 / IS-1230 / IS-1232.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/iodesystems/zdx-go/internal/envagentdaemon"
)

// Version is overridable at link time via -ldflags "-X main.Version=..."
// so deploy packaging can stamp the build sha in. The daemon surfaces
// this on every heartbeat so operators can see which env is on which
// build.
var Version = "dev"

func main() {
	server := flag.String("server", "http://localhost:7600", "dx-server base URL")
	envName := flag.String("env-name", "", "environment name this daemon represents (required, e.g. production)")
	agentID := flag.String("agent-id", "", "agent identifier (default: <hostname>-envd)")
	apiKey := flag.String("api-key", "", "API key (env ZDX_API_KEY)")
	manifestPath := flag.String("manifest-path", "/etc/zdx/deployed.manifest", "path to file naming the currently-deployed commit (empty = unknown)")
	heartbeat := flag.Duration("heartbeat-interval", 15*time.Second, "heartbeat send interval")
	flag.Parse()

	if *envName == "" {
		log.Fatal("--env-name required")
	}
	if *apiKey == "" {
		*apiKey = os.Getenv("ZDX_API_KEY")
	}
	if *apiKey == "" {
		log.Fatal("--api-key or ZDX_API_KEY required")
	}
	if *agentID == "" {
		hostname, _ := os.Hostname()
		*agentID = fmt.Sprintf("%s-envd", hostname)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf("dx-envd starting: env=%s id=%s server=%s version=%s", *envName, *agentID, *server, Version)

	d := &envagentdaemon.Daemon{
		ServerURL:         *server,
		EnvName:           *envName,
		AgentID:           *agentID,
		APIKey:            *apiKey,
		Version:           Version,
		OS:                runtime.GOOS,
		ManifestPath:      *manifestPath,
		HeartbeatInterval: *heartbeat,
	}

	if err := d.RunForever(ctx); err != nil {
		log.Fatalf("daemon: %v", err)
	}
}
