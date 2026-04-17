package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerCodeRefRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-issue", Method: http.MethodPost, Path: "/api/dx/code-refs/issue/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				IssueID   string  `json:"issue_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := in.Body.IssueID
			if _, ferr := h.Q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); ferr != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found: "+issueID)
			}
			ref, err := h.Q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := h.Q.AttachCodeRefToIssue(ctx, db.AttachCodeRefToIssueParams{IssueID: issueID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-issue", Method: http.MethodPost, Path: "/api/dx/code-refs/issue/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				IssueID   string `json:"issue_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := h.Q.DetachCodeRefFromIssue(ctx, db.DetachCodeRefFromIssueParams{IssueID: in.Body.IssueID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-issue", Method: http.MethodGet, Path: "/api/dx/code-refs/issue"},
		func(ctx context.Context, in *struct {
			Slug    string `query:"slug" required:"true"`
			IssueID string `query:"issue_id" required:"true"`
		}) (*struct {
			Body struct {
				Refs []CodeRefItem `json:"refs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := h.Q.ListCodeRefsByIssue(ctx, in.IssueID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct {
				Body struct {
					Refs []CodeRefItem `json:"refs"`
				}
			}{
				Body: struct {
					Refs []CodeRefItem `json:"refs"`
				}{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-task", Method: http.MethodPost, Path: "/api/dx/code-refs/task/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				TaskID    string  `json:"task_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			ref, err := h.Q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := h.Q.AttachCodeRefToTask(ctx, db.AttachCodeRefToTaskParams{TaskID: in.Body.TaskID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-task", Method: http.MethodPost, Path: "/api/dx/code-refs/task/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				TaskID    string `json:"task_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := h.Q.DetachCodeRefFromTask(ctx, db.DetachCodeRefFromTaskParams{TaskID: in.Body.TaskID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-task", Method: http.MethodGet, Path: "/api/dx/code-refs/task"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			TaskID string `query:"task_id" required:"true"`
		}) (*struct {
			Body struct {
				Refs []CodeRefItem `json:"refs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := h.Q.ListCodeRefsByTask(ctx, in.TaskID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct {
				Body struct {
					Refs []CodeRefItem `json:"refs"`
				}
			}{
				Body: struct {
					Refs []CodeRefItem `json:"refs"`
				}{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-test", Method: http.MethodPost, Path: "/api/dx/code-refs/test/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				TestID    int32   `json:"test_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			ref, err := h.Q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := h.Q.AttachCodeRefToTest(ctx, db.AttachCodeRefToTestParams{TestID: in.Body.TestID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-test", Method: http.MethodPost, Path: "/api/dx/code-refs/test/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				TestID    int32  `json:"test_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := h.Q.DetachCodeRefFromTest(ctx, db.DetachCodeRefFromTestParams{TestID: in.Body.TestID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-test", Method: http.MethodGet, Path: "/api/dx/code-refs/test"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			TestID int32  `query:"test_id" required:"true"`
		}) (*struct {
			Body struct {
				Refs []CodeRefItem `json:"refs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := h.Q.ListCodeRefsByTest(ctx, in.TestID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct {
				Body struct {
					Refs []CodeRefItem `json:"refs"`
				}
			}{
				Body: struct {
					Refs []CodeRefItem `json:"refs"`
				}{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "attach-code-ref-to-spec", Method: http.MethodPost, Path: "/api/dx/code-refs/spec/attach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string  `json:"slug"`
				SpecID    int32   `json:"spec_id"`
				FilePath  string  `json:"file_path"`
				GitHash   *string `json:"git_hash,omitempty"`
				LineStart *int32  `json:"line_start,omitempty"`
				LineEnd   *int32  `json:"line_end,omitempty"`
				Note      *string `json:"note,omitempty"`
			}
		}) (*struct{ Body CodeRefItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			ref, err := h.Q.CreateCodeRef(ctx, db.CreateCodeRefParams{
				ProjectID: p.ID,
				FilePath:  in.Body.FilePath,
				GitHash:   ptrStr(in.Body.GitHash),
				LineStart: ptrInt32(in.Body.LineStart),
				LineEnd:   ptrInt32(in.Body.LineEnd),
				Note:      ptrStr(in.Body.Note),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if err := h.Q.AttachCodeRefToSpec(ctx, db.AttachCodeRefToSpecParams{SpecID: in.Body.SpecID, CodeRefID: ref.ID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body CodeRefItem }{Body: toCodeRefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "detach-code-ref-from-spec", Method: http.MethodPost, Path: "/api/dx/code-refs/spec/detach"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				SpecID    int32  `json:"spec_id"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := h.Q.DetachCodeRefFromSpec(ctx, db.DetachCodeRefFromSpecParams{SpecID: in.Body.SpecID, CodeRefID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-code-refs-for-spec", Method: http.MethodGet, Path: "/api/dx/code-refs/spec"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			SpecID int32  `query:"spec_id" required:"true"`
		}) (*struct {
			Body struct {
				Refs []CodeRefItem `json:"refs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := h.Q.ListCodeRefsBySpec(ctx, in.SpecID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]CodeRefItem, len(rows))
			for i, r := range rows {
				out[i] = toCodeRefItem(r)
			}
			return &struct {
				Body struct {
					Refs []CodeRefItem `json:"refs"`
				}
			}{
				Body: struct {
					Refs []CodeRefItem `json:"refs"`
				}{Refs: out},
			}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "read-code-snippet", Method: http.MethodGet, Path: "/api/dx/code-refs/snippet"},
		func(ctx context.Context, in *struct {
			Slug  string `query:"slug" required:"true"`
			Path  string `query:"path" required:"true"`
			Hash  string `query:"hash"`
			Start int    `query:"start"`
			End   int    `query:"end"`
			Pad   int    `query:"pad"`
		}) (*struct{ Body SnippetResult }, error) {
			if _, err := getProject(ctx, h.Q, in.Slug); err != nil {
				return nil, err
			}
			if !h.Features.HasProjectGitConfig {
				return nil, apiErr(http.StatusNotImplemented, "project git config not supported in this build")
			}
			row, err := h.Q.GetProjectGitConfig(ctx, in.Slug)
			if err != nil || row.GitUrl == "" {
				return nil, apiErr(http.StatusUnprocessableEntity, "git URL not configured for project "+in.Slug)
			}
			gitURL := row.GitUrl
			if row.GitToken != "" && strings.HasPrefix(gitURL, "https://") {
				gitURL = "https://" + row.GitToken + "@" + strings.TrimPrefix(gitURL, "https://")
			}
			branch := row.GitBranch
			if branch == "" {
				branch = "main"
			}
			dir := h.Reconciler.RepoDir(in.Slug)
			pad := in.Pad
			if in.Pad == 0 {
				pad = 3
			}
			res, err := ReadSnippet(dir, gitURL, branch, in.Hash, in.Path, in.Start, in.End, pad)
			if err != nil {
				var se *SnippetError
				if errors.As(err, &se) {
					switch se.Code {
					case SnippetErrBadPath:
						return nil, apiErr(http.StatusBadRequest, se.Msg)
					case SnippetErrHashMissing, SnippetErrFileMissing:
						return nil, apiErr(http.StatusNotFound, se.Msg)
					default:
						return nil, apiErr(http.StatusInternalServerError, se.Msg)
					}
				}
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body SnippetResult }{Body: *res}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-code-ref", Method: http.MethodPost, Path: "/api/dx/code-refs/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.DeleteCodeRef(ctx, db.DeleteCodeRefParams{ProjectID: p.ID, ID: in.Body.CodeRefID}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body OKBody }{Body: OKBody{OK: true}}, nil
		})
}

// ── Model → response converter ────────────────────────────────────────────

func toCodeRefItem(r db.ZdxCodeRef) CodeRefItem {
	return CodeRefItem{
		ID:        r.ID,
		FilePath:  r.FilePath,
		GitHash:   r.GitHash,
		LineStart: r.LineStart,
		LineEnd:   r.LineEnd,
		Note:      r.Note,
		CreatedAt: fmtTS(r.CreatedAt),
	}
}
