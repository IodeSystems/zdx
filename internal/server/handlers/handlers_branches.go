package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// IS-825: priority cutoff for auto-generated backport tasks. Issues at this
// numeric priority or lower (1=must, 2=should) backport by default. Hard-coded
// for now; promote to project setting if/when must-only or all-priorities
// becomes a real ask.
const backportPriorityCutoff = 2

type VersionBranchItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Legacy 'dev'/'named' value mapped from db role; full rename is IS-967.
	Type      string `json:"type"`
	Semver    string `json:"semver,omitempty"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// CreateVersionBranchResult mirrors VersionBranchItem and adds the count of
// auto-generated backport tasks (IS-825 trigger 2) so callers — notably the
// `dx branch cut` CLI — can surface "created v1.0.x; auto-generated N backport
// tasks" to the operator without an extra round-trip.
type CreateVersionBranchResult struct {
	VersionBranchItem
	BackportTasksCreated int `json:"backport_tasks_created"`
}

// createBackportTask inserts one auto-generated backport task for sourceIssue
// against targetBranch. It is shared between IS-825's two triggers — branch
// cut (this file) and dev-resolution (handlers_issues.go, TK-1532). Returns
// false (no error) when the task is skipped for idempotency: a non-done task
// already links the same (issue, target_branch). Returns false + error when
// inputs are degenerate (target == 'dev' / empty issue id) — callers treat
// that as a programmer bug, not a runtime condition.
func (h *Handler) createBackportTask(ctx context.Context, projectID int32, sourceIssueID, sourceIssueTitle, targetBranch, reason string) (string, bool, error) {
	if targetBranch == "" || targetBranch == "dev" {
		return "", false, fmt.Errorf("createBackportTask: target_branch must be a named branch, got %q", targetBranch)
	}
	if sourceIssueID == "" {
		return "", false, fmt.Errorf("createBackportTask: source issue id required")
	}
	existing, err := h.Q.CountOpenBackportTasks(ctx, db.CountOpenBackportTasksParams{
		ProjectID:    projectID,
		Issue:        sourceIssueID,
		TargetBranch: targetBranch,
	})
	if err != nil {
		return "", false, err
	}
	if existing > 0 {
		return "", false, nil
	}
	id, err := h.Q.NextTaskID(ctx)
	if err != nil {
		return "", false, err
	}
	title := fmt.Sprintf("Backport %s to %s", sourceIssueID, targetBranch)
	body := fmt.Sprintf("Backport the dev resolution of %s (%q) to branch %s. Suggested worktree branch: %s.", sourceIssueID, sourceIssueTitle, targetBranch, targetBranch)
	if _, err := h.Q.CreateTask(ctx, db.CreateTaskParams{
		ID:           id,
		ProjectID:    projectID,
		Title:        title,
		Text:         body,
		Issue:        sourceIssueID,
		Status:       "ready",
		Reason:       reason,
		TargetBranch: targetBranch,
	}); err != nil {
		return "", false, err
	}
	return id, true, nil
}

type VersionBranchDetail struct {
	VersionBranchItem
	OpenCount     int `json:"open_count"`
	ResolvedCount int `json:"resolved_count"`
}

// roleToLegacyType maps the new role enum back to the legacy type values
// the API still publishes. Values outside the legacy pair pass through
// unchanged so future roles surface verbatim once IS-967 widens the API.
func roleToLegacyType(role string) string {
	switch role {
	case "named-release":
		return "named"
	default:
		return role
	}
}

func versionBranchItemFrom(id int64, name, role string, semver pgtype.Text, status string, createdAt pgtype.Timestamptz) VersionBranchItem {
	sv := ""
	if semver.Valid {
		sv = semver.String
	}
	return VersionBranchItem{
		ID:        id,
		Name:      name,
		Type:      roleToLegacyType(role),
		Semver:    sv,
		Status:    status,
		CreatedAt: fmtTS(createdAt),
	}
}

func (h *Handler) registerBranchRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "create-version-branch", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/branches"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Body struct {
				Name   string `json:"name"`
				Semver string `json:"semver,omitempty"`
			}
		}) (*struct{ Body CreateVersionBranchResult }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.Name == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "name is required")
			}
			semver := pgtype.Text{}
			if in.Body.Semver != "" {
				semver = pgtype.Text{String: in.Body.Semver, Valid: true}
			}
			row, err := h.Q.CreateVersionBranch(ctx, db.CreateVersionBranchParams{
				ProjectID: p.ID,
				Name:      in.Body.Name,
				Role:      "named-release",
				Semver:    semver,
				Status:    "active",
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			// IS-825 trigger 2: enumerate qualifying open dev issues and
			// auto-generate one backport task per issue against the new
			// branch. Failure here does not roll the branch back — the
			// branch is the canonical artifact and operators can re-run
			// the auto-fill out-of-band if it partially fails.
			eligible, lerr := h.Q.ListOpenIssuesEligibleForBackport(ctx, db.ListOpenIssuesEligibleForBackportParams{
				ProjectID:   p.ID,
				MaxPriority: backportPriorityCutoff,
			})
			created := 0
			if lerr == nil {
				for _, iss := range eligible {
					_, made, cerr := h.createBackportTask(ctx, p.ID, iss.ID, iss.Title, row.Name, "auto-generated by IS-825 trigger 2 on branch cut")
					if cerr != nil {
						continue
					}
					if made {
						created++
					}
				}
			}
			if created > 0 {
				h.refreshQueueAsync(p.ID)
			}
			result := CreateVersionBranchResult{
				VersionBranchItem:    versionBranchItemFrom(row.ID, row.Name, row.Role, row.Semver, row.Status, row.CreatedAt),
				BackportTasksCreated: created,
			}
			return &struct{ Body CreateVersionBranchResult }{Body: result}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-version-branches", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/branches"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Branches []VersionBranchItem `json:"branches"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListVersionBranches(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			out := make([]VersionBranchItem, len(rows))
			for i, r := range rows {
				out[i] = versionBranchItemFrom(r.ID, r.Name, r.Role, r.Semver, r.Status, r.CreatedAt)
			}
			return &struct {
				Body struct {
					Branches []VersionBranchItem `json:"branches"`
				}
			}{Body: struct {
				Branches []VersionBranchItem `json:"branches"`
			}{Branches: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "show-version-branch", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/branches/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
		}) (*struct{ Body VersionBranchDetail }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetVersionBranchByName(ctx, db.GetVersionBranchByNameParams{
				ProjectID: p.ID,
				Name:      in.Name,
			})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "branch not found: "+in.Name)
			}
			issues, err := h.Q.ListIssues(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			var openCount, resolvedCount int
			for _, iss := range issues {
				if iss.TargetBranch != in.Name {
					continue
				}
				if iss.Status == "open" || iss.Status == "wip" {
					openCount++
				} else if iss.Status == "closed" {
					resolvedCount++
				}
			}
			detail := VersionBranchDetail{
				VersionBranchItem: versionBranchItemFrom(row.ID, row.Name, row.Role, row.Semver, row.Status, row.CreatedAt),
				OpenCount:         openCount,
				ResolvedCount:     resolvedCount,
			}
			return &struct{ Body VersionBranchDetail }{Body: detail}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "mark-version-branch-eol", Method: http.MethodPatch, Path: "/api/dx/projects/{slug}/branches/{name}/eol"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.MarkVersionBranchEOL(ctx, db.MarkVersionBranchEOLParams{
				ProjectID: p.ID,
				Name:      in.Name,
			}); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}
