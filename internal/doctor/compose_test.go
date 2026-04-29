package doctor

import (
	"os"
	"strings"
	"testing"
)

func TestRenderDevCompose(t *testing.T) {
	tests := []struct {
		name     string
		stack    Stack
		contains []string
		missing  []string
	}{
		{
			name:  "go + postgres + valkey",
			stack: Stack{HasGo: true, GoVersion: "1.25", HasPostgres: true, HasValkey: true},
			contains: []string{
				"services:",
				"  dev:",
				"dockerfile: dev.Dockerfile",
				".:/workspace",
				"DATABASE_URL=postgres://dev:dev@db:5432/dev",
				"REDIS_URL=redis://valkey:6379",
				"depends_on:",
				"  db:",
				"image: pgvector/pgvector:pg16",
				"POSTGRES_USER=dev",
				"  valkey:",
				"image: valkey/valkey:8",
			},
		},
		{
			name:  "go + postgres only",
			stack: Stack{HasGo: true, GoVersion: "1.25", HasPostgres: true},
			contains: []string{
				"DATABASE_URL=postgres://dev:dev@db:5432/dev",
				"  db:",
				"image: pgvector/pgvector:pg16",
			},
			missing: []string{
				"REDIS_URL",
				"valkey",
			},
		},
		{
			name:  "go only (no services)",
			stack: Stack{HasGo: true, GoVersion: "1.25"},
			contains: []string{
				"services:",
				"  dev:",
				"dockerfile: dev.Dockerfile",
				".:/workspace",
			},
			missing: []string{
				"DATABASE_URL",
				"REDIS_URL",
				"depends_on",
				"db:",
				"valkey:",
			},
		},
		{
			name:  "empty stack",
			stack: Stack{},
			contains: []string{
				"services:",
				"  dev:",
				"dockerfile: dev.Dockerfile",
			},
			missing: []string{
				"DATABASE_URL",
				"REDIS_URL",
				"depends_on",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDevCompose(tt.stack)
			for _, s := range tt.contains {
				if !strings.Contains(got, s) {
					t.Errorf("expected output to contain %q, got:\n%s", s, got)
				}
			}
			for _, s := range tt.missing {
				if strings.Contains(got, s) {
					t.Errorf("expected output NOT to contain %q, got:\n%s", s, got)
				}
			}
		})
	}
}

func TestDetectStackValkey(t *testing.T) {
	t.Run("go-redis detected as valkey", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n\nrequire github.com/redis/go-redis/v9 v9.0.0\n")
		s := DetectStack(dir)
		if !s.HasValkey {
			t.Fatalf("expected valkey detection for go-redis, got %+v", s)
		}
	})

	t.Run("valkey-go detected", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n\nrequire github.com/valkey-io/valkey-go v1.0.0\n")
		s := DetectStack(dir)
		if !s.HasValkey {
			t.Fatalf("expected valkey detection for valkey-go, got %+v", s)
		}
	})

	t.Run("no valkey in plain go project", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n")
		s := DetectStack(dir)
		if s.HasValkey {
			t.Fatalf("expected no valkey detection, got %+v", s)
		}
	})
}

func TestScaffoldDevContainer(t *testing.T) {
	t.Run("creates both files when missing", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n")

		if err := ScaffoldDevContainer(); err != nil {
			t.Fatalf("ScaffoldDevContainer: %v", err)
		}

		dockerfile, err := os.ReadFile("dev.Dockerfile")
		if err != nil {
			t.Fatalf("dev.Dockerfile: %v", err)
		}
		if !strings.Contains(string(dockerfile), "FROM golang:1.25-bookworm") {
			t.Errorf("dev.Dockerfile missing go base image, got:\n%s", dockerfile)
		}

		compose, err := os.ReadFile("docker-compose.dev.yml")
		if err != nil {
			t.Fatalf("docker-compose.dev.yml: %v", err)
		}
		if !strings.Contains(string(compose), "dockerfile: dev.Dockerfile") {
			t.Errorf("compose missing dockerfile ref, got:\n%s", compose)
		}
	})

	t.Run("renders postgres + valkey services from detected stack", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n\nrequire (\n\tgithub.com/pgvector/pgvector-go v0.1.0\n\tgithub.com/redis/go-redis/v9 v9.0.0\n)\n")

		if err := ScaffoldDevContainer(); err != nil {
			t.Fatalf("ScaffoldDevContainer: %v", err)
		}

		compose, err := os.ReadFile("docker-compose.dev.yml")
		if err != nil {
			t.Fatalf("docker-compose.dev.yml: %v", err)
		}
		want := []string{
			"DATABASE_URL=postgres://dev:dev@db:5432/dev",
			"REDIS_URL=redis://valkey:6379",
			"image: pgvector/pgvector:pg16",
			"image: valkey/valkey:8",
		}
		for _, s := range want {
			if !strings.Contains(string(compose), s) {
				t.Errorf("compose missing %q, got:\n%s", s, compose)
			}
		}
	})

	t.Run("does not overwrite canonical-named existing files", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n")
		const sentinelDocker = "PRE-EXISTING-DOCKERFILE-MARKER"
		const sentinelCompose = "PRE-EXISTING-COMPOSE-MARKER"
		write(t, dir, "dev.Dockerfile", sentinelDocker)
		write(t, dir, "docker-compose.dev.yml", sentinelCompose)

		if err := ScaffoldDevContainer(); err != nil {
			t.Fatalf("ScaffoldDevContainer: %v", err)
		}

		dockerfile, err := os.ReadFile("dev.Dockerfile")
		if err != nil {
			t.Fatalf("dev.Dockerfile: %v", err)
		}
		if string(dockerfile) != sentinelDocker {
			t.Errorf("dev.Dockerfile was overwritten, got:\n%s", dockerfile)
		}

		compose, err := os.ReadFile("docker-compose.dev.yml")
		if err != nil {
			t.Fatalf("docker-compose.dev.yml: %v", err)
		}
		if string(compose) != sentinelCompose {
			t.Errorf("docker-compose.dev.yml was overwritten, got:\n%s", compose)
		}
	})

	t.Run("recognizes alt-named existing files and skips canonical scaffold", func(t *testing.T) {
		dir := t.TempDir()
		t.Chdir(dir)
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n")
		write(t, dir, "Dockerfile.dev", "PRE-EXISTING-ALT-DOCKERFILE")
		write(t, dir, "docker-compose.dev.yaml", "PRE-EXISTING-ALT-COMPOSE")

		if err := ScaffoldDevContainer(); err != nil {
			t.Fatalf("ScaffoldDevContainer: %v", err)
		}

		if _, err := os.Stat("dev.Dockerfile"); !os.IsNotExist(err) {
			t.Errorf("dev.Dockerfile should not be created when Dockerfile.dev exists, stat err=%v", err)
		}
		if _, err := os.Stat("docker-compose.dev.yml"); !os.IsNotExist(err) {
			t.Errorf("docker-compose.dev.yml should not be created when docker-compose.dev.yaml exists, stat err=%v", err)
		}
	})
}
