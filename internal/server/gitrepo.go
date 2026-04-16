package server

import (
	"os"
	"path/filepath"
)

// RepoDir returns the local clone path for a project.
func (s *Server) RepoDir(slug string) string {
	base := os.Getenv("REPO_DIR")
	if base == "" {
		if zdxHome := os.Getenv("ZDX_HOME"); zdxHome != "" {
			base = zdxHome + "/data/repos"
		} else {
			base = "repos"
		}
	}
	return filepath.Join(base, slug)
}
