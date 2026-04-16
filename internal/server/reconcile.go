package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/server/handlers"
)

func (s *Server) ReconcileBranch(ctx context.Context, projectID int32, slug, targetBranch string) ([]handlers.ReconcileResult, error) {
	row, err := s.q.GetProjectGitConfig(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("project git config not found")
	}
	if row.GitUrl == "" {
		return nil, fmt.Errorf("git URL not configured")
	}
	gitURL := row.GitUrl
	if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
		gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
	}
	dir := s.RepoDir(slug)
	defaultBranch := row.GitBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	if err := handlers.EnsureRepo(dir, gitURL, defaultBranch); err != nil {
		return nil, fmt.Errorf("git: %w", err)
	}

	resolutions, err := s.q.ListResolutionsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}

	var results []handlers.ReconcileResult
	for _, res := range resolutions {
		if res.BranchOfOrigin == targetBranch {
			continue
		}
		commits, err := s.q.ListResolutionCommits(ctx, res.ID)
		if err != nil || len(commits) == 0 {
			continue
		}

		allPresent := true
		for _, c := range commits {
			if !isAncestor(dir, c.Sha, "origin/"+targetBranch) {
				allPresent = false
				break
			}
		}
		if allPresent {
			continue
		}

		matchedSHAs := findEquivalentCommits(dir, commits, targetBranch)
		if len(matchedSHAs) == 0 {
			continue
		}

		var rnd [8]byte
		_, _ = rand.Read(rnd[:])
		newResID := hex.EncodeToString(rnd[:])
		now := pgtype.Timestamptz{Time: time.Now(), Valid: true}
		parentRes := pgtype.Text{String: res.ID, Valid: true}

		_, err = s.q.CreateIssueResolution(ctx, db.CreateIssueResolutionParams{
			ID:                 newResID,
			IssueID:            res.IssueID,
			BranchOfOrigin:     targetBranch,
			ResolvedAt:         now,
			Author:             res.Author,
			Source:             "reconciled",
			ParentResolutionID: parentRes,
		})
		if err != nil {
			continue
		}
		for i, sha := range matchedSHAs {
			_ = s.q.AddResolutionCommit(ctx, db.AddResolutionCommitParams{
				ResolutionID: newResID,
				Sha:          sha,
				Ord:          int32(i),
			})
		}
		results = append(results, handlers.ReconcileResult{
			IssueID:      res.IssueID,
			ResolutionID: newResID,
			MatchedSHAs:  matchedSHAs,
			Source:       "reconciled",
		})
	}
	return results, nil
}

func isAncestor(repoDir, sha, ref string) bool {
	cmd := exec.Command("git", "-C", repoDir, "merge-base", "--is-ancestor", sha, ref)
	cmd.Env = handlers.GitEnv()
	return cmd.Run() == nil
}

func getPatchID(repoDir, sha string) string {
	cmd := exec.Command("git", "-C", repoDir, "show", sha)
	cmd.Env = handlers.GitEnv()
	showOut, err := cmd.Output()
	if err != nil {
		return ""
	}
	patchCmd := exec.Command("git", "-C", repoDir, "patch-id", "--stable")
	patchCmd.Env = handlers.GitEnv()
	patchCmd.Stdin = strings.NewReader(string(showOut))
	out, err := patchCmd.Output()
	if err != nil {
		return ""
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

func findEquivalentCommits(repoDir string, originalCommits []db.ZdxIssueResolutionCommit, targetBranch string) []string {
	origPatchIDs := make(map[string]bool)
	for _, c := range originalCommits {
		pid := getPatchID(repoDir, c.Sha)
		if pid != "" {
			origPatchIDs[pid] = true
		}
	}
	if len(origPatchIDs) == 0 {
		return nil
	}

	cmd := exec.Command("git", "-C", repoDir, "log", "--format=%H", "origin/"+targetBranch, "-100")
	cmd.Env = handlers.GitEnv()
	out, err := cmd.Output()
	if err != nil {
		return nil
	}

	matched := make(map[string]bool)
	var matchedSHAs []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		sha := strings.TrimSpace(line)
		if sha == "" {
			continue
		}
		pid := getPatchID(repoDir, sha)
		if pid != "" && origPatchIDs[pid] && !matched[sha] {
			matched[sha] = true
			matchedSHAs = append(matchedSHAs, sha)
		}
	}

	if len(matchedSHAs) >= len(originalCommits) {
		return matchedSHAs
	}

	if len(originalCommits) > 1 && len(matchedSHAs) == 0 {
		matchedSHAs = findSquashEquivalent(repoDir, originalCommits, targetBranch)
	}

	return matchedSHAs
}

func findSquashEquivalent(repoDir string, originalCommits []db.ZdxIssueResolutionCommit, targetBranch string) []string {
	if len(originalCommits) < 2 {
		return nil
	}
	first := originalCommits[0].Sha
	last := originalCommits[len(originalCommits)-1].Sha

	cmd := exec.Command("git", "-C", repoDir, "diff", first+"^", last)
	cmd.Env = handlers.GitEnv()
	combinedDiff, err := cmd.Output()
	if err != nil || len(combinedDiff) == 0 {
		return nil
	}
	patchCmd := exec.Command("git", "-C", repoDir, "patch-id", "--stable")
	patchCmd.Env = handlers.GitEnv()
	patchCmd.Stdin = strings.NewReader(string(combinedDiff))
	out, err := patchCmd.Output()
	if err != nil {
		return nil
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) == 0 {
		return nil
	}
	combinedPatchID := parts[0]

	logCmd := exec.Command("git", "-C", repoDir, "log", "--format=%H", "origin/"+targetBranch, "-50")
	logCmd.Env = handlers.GitEnv()
	logOut, err := logCmd.Output()
	if err != nil {
		return nil
	}

	for _, line := range strings.Split(strings.TrimSpace(string(logOut)), "\n") {
		sha := strings.TrimSpace(line)
		if sha == "" {
			continue
		}
		pid := getPatchID(repoDir, sha)
		if pid == combinedPatchID {
			return []string{sha}
		}
	}
	return nil
}
