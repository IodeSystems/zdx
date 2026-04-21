package doctor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderDevDockerfile(t *testing.T) {
	tests := []struct {
		name     string
		stack    Stack
		contains []string
		missing  []string
	}{
		{
			name:     "go only",
			stack:    Stack{HasGo: true, GoVersion: "1.25"},
			contains: []string{"FROM golang:1.25-bookworm", "WORKDIR /workspace"},
			missing:  []string{"nodejs", "sqlc", "postgresql-client", "python"},
		},
		{
			name:     "node only",
			stack:    Stack{HasNode: true, NodeVersion: "20"},
			contains: []string{"FROM node:20-bookworm"},
			missing:  []string{"golang:", "sqlc", "python:"},
		},
		{
			name:     "python only",
			stack:    Stack{HasPython: true, PythonMinor: "3.12"},
			contains: []string{"FROM python:3.12-slim"},
			missing:  []string{"golang:", "node:", "sqlc"},
		},
		{
			name:     "go + postgres + sqlc",
			stack:    Stack{HasGo: true, GoVersion: "1.25", HasPostgres: true, HasSqlc: true},
			contains: []string{"FROM golang:1.25-bookworm", "postgresql-client", "sqlc"},
			missing:  []string{"nodejs"},
		},
		{
			name:     "go + node (multi-stack)",
			stack:    Stack{HasGo: true, GoVersion: "1.25", HasNode: true, NodeVersion: "20"},
			contains: []string{"FROM golang:1.25-bookworm", "nodesource.com/setup_20.x", "nodejs"},
			missing:  []string{"FROM node:"},
		},
		{
			name:     "empty stack falls back to debian",
			stack:    Stack{},
			contains: []string{"FROM debian:bookworm-slim", "WORKDIR /workspace"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RenderDevDockerfile(tt.stack)
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

func TestDetectStack(t *testing.T) {
	t.Run("go-only from go.mod", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module example.com/x\n\ngo 1.24\n")
		s := DetectStack(dir)
		if !s.HasGo || s.GoVersion != "1.24" {
			t.Fatalf("expected Go 1.24, got %+v", s)
		}
		if s.HasNode || s.HasPython || s.HasSqlc || s.HasPostgres {
			t.Fatalf("unexpected non-Go signals: %+v", s)
		}
	})

	t.Run("go with pgx postgres driver", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n\nrequire github.com/jackc/pgx/v5 v5.0.0\n")
		s := DetectStack(dir)
		if !s.HasPostgres {
			t.Fatalf("expected postgres detection, got %+v", s)
		}
	})

	t.Run("node engines override", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "package.json", `{"engines":{"node":">=22.1.0"}}`)
		s := DetectStack(dir)
		if !s.HasNode || s.NodeVersion != "22" {
			t.Fatalf("expected node 22, got %+v", s)
		}
	})

	t.Run("ui/package.json multi-stack", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "go.mod", "module x\n\ngo 1.25\n")
		write(t, filepath.Join(dir, "ui"), "package.json", `{}`)
		s := DetectStack(dir)
		if !s.HasGo || !s.HasNode {
			t.Fatalf("expected multi-stack, got %+v", s)
		}
		if s.NodeVersion != "20" {
			t.Fatalf("expected node fallback 20, got %q", s.NodeVersion)
		}
	})

	t.Run("sqlc.yaml present", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "sqlc.yaml", "version: \"2\"\n")
		s := DetectStack(dir)
		if !s.HasSqlc {
			t.Fatalf("expected sqlc detection")
		}
	})

	t.Run("python via pyproject", func(t *testing.T) {
		dir := t.TempDir()
		write(t, dir, "pyproject.toml", "[project]\npython_requires = \">=3.11\"\n")
		s := DetectStack(dir)
		if !s.HasPython || s.PythonMinor != "3.11" {
			t.Fatalf("expected python 3.11, got %+v", s)
		}
	})
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
