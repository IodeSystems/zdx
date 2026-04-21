package doctor

import (
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
