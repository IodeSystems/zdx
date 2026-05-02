package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/doctor"
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
	Type             string `json:"type"`
	Role             string `json:"role,omitempty"`
	SourceBranchName string `json:"source_branch_name,omitempty"`
	Semver           string `json:"semver,omitempty"`
	Status           string `json:"status"`
	CreatedAt        string `json:"created_at"`
	MergeStyle       string `json:"merge_style,omitempty"`
	RequiredChecks   string `json:"required_checks,omitempty"`
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

// BranchDoctorRungResult mirrors the branching_strategy_appropriate doctor
// rung so the UI can surface project branching health inline without shelling
// out to `dx doctor`. Status, message, and proposal text are kept intentionally
// close to the CLI rung output; the classification field lets the UI tailor
// any nudge copy. The DeployEventCount input the CLI uses for tool/site rung-1
// fails is intentionally omitted here — this endpoint is the smallest surface
// that unblocks the UI (TK-1668); CLI `dx doctor` remains authoritative for
// deploy-aware nudges.
type BranchDoctorRungResult struct {
	Status         string `json:"status"`
	CurrentRung    int    `json:"current_rung"`
	Message        string `json:"message"`
	Proposal       string `json:"proposal,omitempty"`
	Classification string `json:"classification"`
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

// branchSeeder is the slice of *db.Queries that seedClassificationBranches
// needs — extracted into an interface so the seed loop can be unit-tested
// against a fake.
type branchSeeder interface {
	UpsertVersionBranchIfMissing(ctx context.Context, arg db.UpsertVersionBranchIfMissingParams) error
}

// seedClassificationBranches writes one row per BranchSeed for the project's
// classification, idempotently (the underlying query is INSERT … ON CONFLICT
// DO NOTHING keyed on (project_id, name)). Empty / unknown classifications
// produce zero seeds — the caller still gets a successful response so dx init
// can run before classification is set without erroring out.
func seedClassificationBranches(ctx context.Context, q branchSeeder, projectID int32, classification string) error {
	for _, seed := range doctor.ClassificationBranchSeeds(doctor.Classification(classification)) {
		src := pgtype.Text{}
		if seed.SourceBranchName != "" {
			src = pgtype.Text{String: seed.SourceBranchName, Valid: true}
		}
		if err := q.UpsertVersionBranchIfMissing(ctx, db.UpsertVersionBranchIfMissingParams{
			ProjectID:        projectID,
			Name:             seed.Name,
			Role:             seed.Role,
			SourceBranchName: src,
		}); err != nil {
			return err
		}
	}
	return nil
}

func versionBranchItemFrom(id int64, name, role string, semver pgtype.Text, status string, createdAt pgtype.Timestamptz, sourceBranchName, mergeStyle, requiredChecks pgtype.Text) VersionBranchItem {
	sv := ""
	if semver.Valid {
		sv = semver.String
	}
	src := ""
	if sourceBranchName.Valid {
		src = sourceBranchName.String
	}
	ms := ""
	if mergeStyle.Valid {
		ms = mergeStyle.String
	}
	rc := ""
	if requiredChecks.Valid {
		rc = requiredChecks.String
	}
	return VersionBranchItem{
		ID:               id,
		Name:             name,
		Type:             roleToLegacyType(role),
		Role:             role,
		SourceBranchName: src,
		Semver:           sv,
		Status:           status,
		CreatedAt:        fmtTS(createdAt),
		MergeStyle:       ms,
		RequiredChecks:   rc,
	}
}

func (h *Handler) registerBranchRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "create-version-branch", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/branches"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Body struct {
				Name             string `json:"name"`
				Semver           string `json:"semver,omitempty"`
				Role             string `json:"role,omitempty"`
				SourceBranchName string `json:"source_branch_name,omitempty"`
			}
		}) (*struct{ Body CreateVersionBranchResult }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.Name == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "name is required")
			}
			role := in.Body.Role
			if role == "" {
				role = "named-release"
			}
			switch role {
			case "rolling-release", "dev", "pr-target", "named-release":
			default:
				return nil, apiErr(http.StatusUnprocessableEntity, "role must be one of rolling-release, dev, pr-target, named-release")
			}
			semver := pgtype.Text{}
			if in.Body.Semver != "" {
				semver = pgtype.Text{String: in.Body.Semver, Valid: true}
			}
			source := pgtype.Text{}
			if in.Body.SourceBranchName != "" {
				source = pgtype.Text{String: in.Body.SourceBranchName, Valid: true}
			}
			row, err := h.Q.CreateVersionBranch(ctx, db.CreateVersionBranchParams{
				ProjectID:        p.ID,
				Name:             in.Body.Name,
				Role:             role,
				Semver:           semver,
				Status:           "active",
				SourceBranchName: source,
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
				VersionBranchItem:    versionBranchItemFrom(row.ID, row.Name, row.Role, row.Semver, row.Status, row.CreatedAt, row.SourceBranchName, row.MergeStyle, row.RequiredChecks),
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
				out[i] = versionBranchItemFrom(r.ID, r.Name, r.Role, r.Semver, r.Status, r.CreatedAt, r.SourceBranchName, r.MergeStyle, r.RequiredChecks)
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
				VersionBranchItem: versionBranchItemFrom(row.ID, row.Name, row.Role, row.Semver, row.Status, row.CreatedAt, row.SourceBranchName, row.MergeStyle, row.RequiredChecks),
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

	huma.Register(api, huma.Operation{OperationID: "seed-version-branches", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/branches/seed"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
		}) (*struct{}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := seedClassificationBranches(ctx, h.Q, p.ID, p.Classification); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{}{}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-branch-doctor-rung", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/branches/doctor-rung"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
		}) (*struct{ Body BranchDoctorRungResult }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListVersionBranches(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			var hasDev, hasPRTarget, hasNamedRelease, hasRollingRelease bool
			for _, r := range rows {
				switch r.Role {
				case "dev":
					hasDev = true
				case "pr-target":
					hasPRTarget = true
				case "named-release":
					hasNamedRelease = true
				case "rolling-release":
					hasRollingRelease = true
				}
			}
			_ = hasRollingRelease // tracked for future rungs; current logic does not branch on it.

			rung := 1
			switch {
			case hasDev && hasNamedRelease:
				rung = 4
			case hasDev:
				rung = 3
			case hasPRTarget:
				rung = 2
			}

			class := p.Classification
			if class == "" {
				class = "tool"
			}

			rungLabel := fmt.Sprintf("rung %d", rung)
			if rung == 1 {
				rungLabel = "rung 1 (no version-branch rows)"
			}

			out := BranchDoctorRungResult{
				CurrentRung:    rung,
				Classification: class,
				Status:         "pass",
			}
			switch class {
			case "library":
				out.Message = fmt.Sprintf("library at %s — branching strategy not gated", rungLabel)
			case "service", "saas":
				if rung == 1 {
					out.Status = "fail"
					out.Message = fmt.Sprintf("%s at rung 1 — no dev branch row; deploys lack a buffer", class)
					out.Proposal = "Advance to rung 3: add a dev branch and wire main.source_branch_name=dev for a deploy buffer. " +
						"Run `dx branch add-role dev --name dev` and `dx branch set-source main --source dev`."
				} else {
					out.Message = fmt.Sprintf("%s at %s", class, rungLabel)
				}
			case "tool", "site":
				// CLI doctor also fails this case at rung 1 when DeployEventCount > 0; the API
				// omits that gate (no deploy lookup) — see BranchDoctorRungResult docstring.
				out.Message = fmt.Sprintf("%s at %s", class, rungLabel)
			default:
				out.Message = fmt.Sprintf("%s at %s", class, rungLabel)
			}
			return &struct{ Body BranchDoctorRungResult }{Body: out}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-version-branch-source", Method: http.MethodPatch, Path: "/api/dx/projects/{slug}/branches/{name}/source"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
			Body struct {
				SourceBranchName string `json:"source_branch_name"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			src := pgtype.Text{}
			if in.Body.SourceBranchName != "" {
				src = pgtype.Text{String: in.Body.SourceBranchName, Valid: true}
			}
			if err := h.Q.UpdateVersionBranchSource(ctx, db.UpdateVersionBranchSourceParams{
				ProjectID:        p.ID,
				Name:             in.Name,
				SourceBranchName: src,
			}); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-version-branch-settings", Method: http.MethodPatch, Path: "/api/dx/projects/{slug}/branches/{name}/settings"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
			Body struct {
				MergeStyle     *string `json:"merge_style,omitempty"`
				RequiredChecks *string `json:"required_checks,omitempty"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.MergeStyle != nil {
				switch *in.Body.MergeStyle {
				case "squash", "merge", "rebase", "":
				default:
					return nil, apiErr(http.StatusUnprocessableEntity, "merge_style must be one of squash, merge, rebase")
				}
			}
			ms := pgtype.Text{}
			if in.Body.MergeStyle != nil && *in.Body.MergeStyle != "" {
				ms = pgtype.Text{String: *in.Body.MergeStyle, Valid: true}
			}
			rc := pgtype.Text{}
			if in.Body.RequiredChecks != nil {
				rc = pgtype.Text{String: *in.Body.RequiredChecks, Valid: true}
			}
			if err := h.Q.UpdateVersionBranchSettings(ctx, db.UpdateVersionBranchSettingsParams{
				ProjectID:      p.ID,
				Name:           in.Name,
				MergeStyle:     ms,
				RequiredChecks: rc,
			}); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-version-branch", Method: http.MethodPatch, Path: "/api/dx/projects/{slug}/branches/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
			Body struct {
				Role             *string `json:"role,omitempty"`
				SourceBranchName *string `json:"source_branch_name,omitempty"`
				AutoSeed         *bool   `json:"auto_seed,omitempty"`
			}
		}) (*struct{ Body VersionBranchItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.Role != nil {
				switch *in.Body.Role {
				case "rolling-release", "dev", "pr-target", "named-release":
				default:
					return nil, apiErr(http.StatusUnprocessableEntity, "role must be one of rolling-release, dev, pr-target, named-release")
				}
			}
			if in.Body.Role != nil || in.Body.AutoSeed != nil {
				r := pgtype.Text{}
				if in.Body.Role != nil {
					r = pgtype.Text{String: *in.Body.Role, Valid: true}
				}
				as := pgtype.Bool{}
				if in.Body.AutoSeed != nil {
					as = pgtype.Bool{Bool: *in.Body.AutoSeed, Valid: true}
				}
				if err := h.Q.UpdateVersionBranch(ctx, db.UpdateVersionBranchParams{
					ProjectID: p.ID,
					Name:      in.Name,
					Role:      r,
					AutoSeed:  as,
				}); err != nil {
					return nil, apiErr(http.StatusInternalServerError, err.Error())
				}
			}
			if in.Body.SourceBranchName != nil {
				src := pgtype.Text{}
				if *in.Body.SourceBranchName != "" {
					src = pgtype.Text{String: *in.Body.SourceBranchName, Valid: true}
				}
				if err := h.Q.UpdateVersionBranchSource(ctx, db.UpdateVersionBranchSourceParams{
					ProjectID:        p.ID,
					Name:             in.Name,
					SourceBranchName: src,
				}); err != nil {
					return nil, apiErr(http.StatusInternalServerError, err.Error())
				}
			}
			row, err := h.Q.GetVersionBranchByName(ctx, db.GetVersionBranchByNameParams{
				ProjectID: p.ID,
				Name:      in.Name,
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			item := versionBranchItemFrom(row.ID, row.Name, row.Role, row.Semver, row.Status, row.CreatedAt, row.SourceBranchName, row.MergeStyle, row.RequiredChecks)
			return &struct{ Body VersionBranchItem }{Body: item}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-version-branch", Method: http.MethodDelete, Path: "/api/dx/projects/{slug}/branches/{name}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug" required:"true"`
			Name string `path:"name" required:"true"`
		}) (*struct{}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			blocking, err := h.Q.ListEnvironmentNamesBoundToBranch(ctx, db.ListEnvironmentNamesBoundToBranchParams{
				ProjectID:  p.ID,
				BranchName: in.Name,
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			if len(blocking) > 0 {
				return nil, apiErr(http.StatusConflict, "blocked by environments: "+strings.Join(blocking, ", "))
			}
			if err := h.Q.DeleteVersionBranch(ctx, db.DeleteVersionBranchParams{
				ProjectID: p.ID,
				Name:      in.Name,
			}); err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{}{}, nil
		})
}
