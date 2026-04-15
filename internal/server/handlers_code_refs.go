package server

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (s *Server) registerCodeRefRoutes(api huma.API) {
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			issueID := in.Body.IssueID
			if _, ferr := s.q.GetIssue(ctx, db.GetIssueParams{ProjectID: p.ID, ID: issueID}); ferr != nil {
				return nil, apiErr(http.StatusNotFound, "issue not found: "+issueID)
			}
			ref, err := s.q.CreateCodeRef(ctx, db.CreateCodeRefParams{
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
			if err := s.q.AttachCodeRefToIssue(ctx, db.AttachCodeRefToIssueParams{IssueID: issueID, CodeRefID: ref.ID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := s.q.DetachCodeRefFromIssue(ctx, db.DetachCodeRefFromIssueParams{IssueID: in.Body.IssueID, CodeRefID: in.Body.CodeRefID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := s.q.ListCodeRefsByIssue(ctx, in.IssueID)
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			ref, err := s.q.CreateCodeRef(ctx, db.CreateCodeRefParams{
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
			if err := s.q.AttachCodeRefToTask(ctx, db.AttachCodeRefToTaskParams{TaskID: in.Body.TaskID, CodeRefID: ref.ID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := s.q.DetachCodeRefFromTask(ctx, db.DetachCodeRefFromTaskParams{TaskID: in.Body.TaskID, CodeRefID: in.Body.CodeRefID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := s.q.ListCodeRefsByTask(ctx, in.TaskID)
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if _, ferr := s.q.GetTestByID(ctx, db.GetTestByIDParams{ProjectID: p.ID, ID: in.Body.TestID}); ferr != nil {
				return nil, apiErr(http.StatusNotFound, "test not found")
			}
			ref, err := s.q.CreateCodeRef(ctx, db.CreateCodeRefParams{
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
			if err := s.q.AttachCodeRefToTest(ctx, db.AttachCodeRefToTestParams{TestID: in.Body.TestID, CodeRefID: ref.ID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			if err := s.q.DetachCodeRefFromTest(ctx, db.DetachCodeRefFromTestParams{TestID: in.Body.TestID, CodeRefID: in.Body.CodeRefID}); err != nil {
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
			p, err := getProject(ctx, s.q, in.Slug)
			if err != nil {
				return nil, err
			}
			_ = p
			rows, err := s.q.ListCodeRefsByTest(ctx, in.TestID)
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

	huma.Register(api, huma.Operation{OperationID: "delete-code-ref", Method: http.MethodPost, Path: "/api/dx/code-refs/delete"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug      string `json:"slug"`
				CodeRefID int32  `json:"code_ref_id"`
			}
		}) (*struct{ Body OKBody }, error) {
			p, err := getProject(ctx, s.q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if err := s.q.DeleteCodeRef(ctx, db.DeleteCodeRefParams{ProjectID: p.ID, ID: in.Body.CodeRefID}); err != nil {
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
