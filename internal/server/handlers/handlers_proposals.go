package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func proposalReviewToken(secret string, projectID int32, title, body string) string {
	hourBucket := time.Now().Unix() / 3600
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "%d\x00%s\x00%s\x00%d", projectID, title, body, hourBucket)
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

func verifyProposalReviewToken(secret, token string, projectID int32, title, body string) bool {
	now := time.Now().Unix() / 3600
	for _, offset := range []int64{0, -1} {
		mac := hmac.New(sha256.New, []byte(secret))
		fmt.Fprintf(mac, "%d\x00%s\x00%s\x00%d", projectID, title, body, now+offset)
		expected := base64.URLEncoding.EncodeToString(mac.Sum(nil))
		if hmac.Equal([]byte(token), []byte(expected)) {
			return true
		}
	}
	return false
}

type ProposalItem struct {
	ID              int32   `json:"id"`
	ProjectID       int32   `json:"project_id"`
	Title           string  `json:"title"`
	Body            string  `json:"body"`
	SourceType      string  `json:"source_type"`
	SourceRef       *string `json:"source_ref,omitempty"`
	Status          string  `json:"status"`
	SnoozedUntil    *string `json:"snoozed_until,omitempty"`
	CreatedBy       string  `json:"created_by"`
	CreatedAt       string  `json:"created_at"`
	UpdatedAt       string  `json:"updated_at"`
	ApprovedIssueID *string `json:"approved_issue_id,omitempty"`
}

type ProposalVersionItem struct {
	ID         int32  `json:"id"`
	ProposalID int32  `json:"proposal_id"`
	Body       string `json:"body"`
	EditedBy   string `json:"edited_by"`
	EditedAt   string `json:"edited_at"`
}

func toProposalItem(p db.ZdxProposal) ProposalItem {
	item := ProposalItem{
		ID:         p.ID,
		ProjectID:  p.ProjectID,
		Title:      p.Title,
		Body:       p.Body,
		SourceType: p.SourceType,
		Status:     p.Status,
		CreatedBy:  p.CreatedBy,
		CreatedAt:  fmtTS(p.CreatedAt),
		UpdatedAt:  fmtTS(p.UpdatedAt),
	}
	if p.SourceRef.Valid {
		item.SourceRef = &p.SourceRef.String
	}
	if p.SnoozedUntil.Valid {
		s := fmtTS(p.SnoozedUntil)
		item.SnoozedUntil = &s
	}
	if p.ApprovedIssueID.Valid {
		item.ApprovedIssueID = &p.ApprovedIssueID.String
	}
	return item
}

func toProposalVersionItem(v db.ZdxProposalVersion) ProposalVersionItem {
	return ProposalVersionItem{
		ID:         v.ID,
		ProposalID: v.ProposalID,
		Body:       v.Body,
		EditedBy:   v.EditedBy,
		EditedAt:   fmtTS(v.EditedAt),
	}
}

func (h *Handler) registerProposalRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-proposals", Method: http.MethodGet, Path: "/api/dx/proposals"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug" required:"true"`
			Status string `query:"status"`
		}) (*struct {
			Body struct {
				Proposals []ProposalItem `json:"proposals"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			status := in.Status
			if status == "" {
				status = "proposed"
			}
			rows, err := h.Q.ListProposals(ctx, db.ListProposalsParams{ProjectID: p.ID, Column2: status})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]ProposalItem, len(rows))
			for i, r := range rows {
				out[i] = toProposalItem(r)
			}
			return &struct {
				Body struct {
					Proposals []ProposalItem `json:"proposals"`
				}
			}{Body: struct {
				Proposals []ProposalItem `json:"proposals"`
			}{Proposals: out}}, nil
		})

	type createProposalBody struct {
		Proposal              *ProposalItem         `json:"proposal,omitempty"`
		Similar               []SimilarProposalItem `json:"similar,omitempty"`
		DuplicatesReviewToken *string               `json:"duplicates_review_token,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "create-proposal", Method: http.MethodPost, Path: "/api/dx/proposals"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug               string  `json:"slug"`
				Title              string  `json:"title"`
				Body               string  `json:"body"`
				SourceType         string  `json:"source_type"`
				SourceRef          *string `json:"source_ref,omitempty"`
				DuplicatesReviewed *string `json:"duplicates_reviewed,omitempty"`
			}
		}) (*struct{ Body createProposalBody }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}

			if in.Body.DuplicatesReviewed != nil {
				if !verifyProposalReviewToken(h.WSSecret, *in.Body.DuplicatesReviewed, p.ID, in.Body.Title, in.Body.Body) {
					return nil, apiErr(http.StatusUnprocessableEntity, "invalid or expired duplicates review token")
				}
			} else {
				similar, _ := h.findSimilarProposals(ctx, p.ID, in.Body.Title+" "+in.Body.Body, 10)
				if len(similar) > 0 {
					token := proposalReviewToken(h.WSSecret, p.ID, in.Body.Title, in.Body.Body)
					return &struct{ Body createProposalBody }{Body: createProposalBody{Similar: similar, DuplicatesReviewToken: &token}}, nil
				}
			}

			createdBy := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, e := h.Q.GetUserByID(ctx, uid); e == nil {
					createdBy = u.Email
				}
			}
			if createdBy == "" {
				createdBy = ctxAgentIDVal(ctx)
			}
			sourceType := in.Body.SourceType
			if sourceType == "" {
				sourceType = "conversation"
			}
			params := db.CreateProposalParams{
				ProjectID:  p.ID,
				Title:      in.Body.Title,
				Body:       in.Body.Body,
				SourceType: sourceType,
				CreatedBy:  createdBy,
			}
			if in.Body.SourceRef != nil {
				params.SourceRef = pgtype.Text{String: *in.Body.SourceRef, Valid: true}
			}
			row, err := h.Q.CreateProposal(ctx, params)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			go h.Emb.UpsertProposal(context.Background(), p.ID, row.ID, row.Title+" "+row.Body)
			item := toProposalItem(row)
			return &struct{ Body createProposalBody }{Body: createProposalBody{Proposal: &item}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "show-proposal", Method: http.MethodGet, Path: "/api/dx/proposals/{id}"},
		func(ctx context.Context, in *struct {
			ID   int32  `path:"id"`
			Slug string `query:"slug" required:"true"`
		}) (*struct {
			Body struct {
				Proposal ProposalItem          `json:"proposal"`
				Versions []ProposalVersionItem `json:"versions"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("proposal %d not found", in.ID))
			}
			versions, _ := h.Q.ListProposalVersions(ctx, in.ID)
			vout := make([]ProposalVersionItem, len(versions))
			for i, v := range versions {
				vout[i] = toProposalVersionItem(v)
			}
			type respBody = struct {
				Proposal ProposalItem          `json:"proposal"`
				Versions []ProposalVersionItem `json:"versions"`
			}
			return &struct{ Body respBody }{Body: respBody{Proposal: toProposalItem(row), Versions: vout}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-proposal", Method: http.MethodPatch, Path: "/api/dx/proposals/{id}"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug  string `json:"slug"`
				Title string `json:"title"`
				Body  string `json:"body"`
			}
		}) (*struct{ Body ProposalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			existing, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("proposal %d not found", in.ID))
			}
			editedBy := ""
			if uid := ctxUserIDVal(ctx); uid != 0 {
				if u, e := h.Q.GetUserByID(ctx, uid); e == nil {
					editedBy = u.Email
				}
			}
			if editedBy == "" {
				editedBy = ctxAgentIDVal(ctx)
			}
			// snapshot previous body as a version before updating
			_, _ = h.Q.CreateProposalVersion(ctx, db.CreateProposalVersionParams{
				ProposalID: in.ID,
				Body:       existing.Body,
				EditedBy:   editedBy,
			})
			row, err := h.Q.UpdateProposal(ctx, db.UpdateProposalParams{
				ProjectID: p.ID,
				ID:        in.ID,
				Title:     in.Body.Title,
				Body:      in.Body.Body,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ProposalItem }{Body: toProposalItem(row)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "approve-proposal", Method: http.MethodPost, Path: "/api/dx/proposals/{id}/approve"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug      string  `json:"slug"`
				IssueType *string `json:"issue_type,omitempty"`
				Priority  *int32  `json:"priority,omitempty"`
			}
		}) (*struct {
			Body struct {
				Proposal ProposalItem `json:"proposal"`
				IssueID  string       `json:"issue_id"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			proposal, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: p.ID, ID: in.ID})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("proposal %d not found", in.ID))
			}
			if proposal.Status == "approved" {
				return nil, apiErr(http.StatusConflict, "proposal already approved")
			}
			issueID, err := h.Q.NextIssueID(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			issueType := "impl"
			if in.Body.IssueType != nil {
				issueType = *in.Body.IssueType
			}
			createParams := db.CreateIssueParams{
				ID:        issueID,
				ProjectID: p.ID,
				Title:     proposal.Title,
				Context:   proposal.Body,
				IssueType: issueType,
				Status:    "open",
			}
			issue, err := h.Q.CreateIssue(ctx, createParams)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			updated, err := h.Q.UpdateProposalStatus(ctx, db.UpdateProposalStatusParams{
				ProjectID:       p.ID,
				ID:              in.ID,
				Status:          "approved",
				SnoozedUntil:    pgtype.Timestamptz{},
				ApprovedIssueID: pgtype.Text{String: issue.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			type respBody = struct {
				Proposal ProposalItem `json:"proposal"`
				IssueID  string       `json:"issue_id"`
			}
			return &struct{ Body respBody }{Body: respBody{Proposal: toProposalItem(updated), IssueID: issue.ID}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reject-proposal", Method: http.MethodPost, Path: "/api/dx/proposals/{id}/reject"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug   string `json:"slug"`
				Reason string `json:"reason,omitempty"`
			}
		}) (*struct{ Body ProposalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if _, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: p.ID, ID: in.ID}); err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("proposal %d not found", in.ID))
			}
			updated, err := h.Q.UpdateProposalStatus(ctx, db.UpdateProposalStatusParams{
				ProjectID:    p.ID,
				ID:           in.ID,
				Status:       "rejected",
				SnoozedUntil: pgtype.Timestamptz{},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if in.Body.Reason != "" {
				actor := ""
				if uid := ctxUserIDVal(ctx); uid != 0 {
					if u, e := h.Q.GetUserByID(ctx, uid); e == nil {
						actor = u.Email
					}
				}
				_, _ = h.Q.AddComment(ctx, db.AddCommentParams{
					ProjectID:  p.ID,
					TargetType: "proposal",
					TargetID:   fmt.Sprintf("%d", in.ID),
					Author:     actor,
					Body:       in.Body.Reason,
				})
			}
			return &struct{ Body ProposalItem }{Body: toProposalItem(updated)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "snooze-proposal", Method: http.MethodPost, Path: "/api/dx/proposals/{id}/snooze"},
		func(ctx context.Context, in *struct {
			ID   int32 `path:"id"`
			Body struct {
				Slug         string `json:"slug"`
				SnoozedUntil string `json:"snoozed_until"`
			}
		}) (*struct{ Body ProposalItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			if _, err := h.Q.GetProposal(ctx, db.GetProposalParams{ProjectID: p.ID, ID: in.ID}); err != nil {
				return nil, apiErr(http.StatusNotFound, fmt.Sprintf("proposal %d not found", in.ID))
			}
			t, err := time.Parse(time.RFC3339, in.Body.SnoozedUntil)
			if err != nil {
				return nil, apiErr(http.StatusBadRequest, "snoozed_until must be RFC3339")
			}
			updated, err := h.Q.UpdateProposalStatus(ctx, db.UpdateProposalStatusParams{
				ProjectID:    p.ID,
				ID:           in.ID,
				Status:       "snoozed",
				SnoozedUntil: pgtype.Timestamptz{Time: t, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body ProposalItem }{Body: toProposalItem(updated)}, nil
		})
}
