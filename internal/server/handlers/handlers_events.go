package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

func (h *Handler) registerEventRoutes(api huma.API) {
	type EventItem struct {
		ID                 int64           `json:"id"`
		TargetType         string          `json:"target_type"`
		TargetID           string          `json:"target_id"`
		ThreadID           *int64          `json:"thread_id,omitempty"`
		EventType          string          `json:"event_type"`
		Author             string          `json:"author"`
		AuthorKind         string          `json:"author_kind"`
		SummaryJson        json.RawMessage `json:"summary_json"`
		DetailJson         json.RawMessage `json:"detail_json"`
		AgentProcessResult json.RawMessage `json:"agent_process_result,omitempty"`
		CreatedAt          string          `json:"created_at"`
	}

	type ThreadItem struct {
		ID          int64  `json:"id"`
		TargetType  string `json:"target_type"`
		TargetID    string `json:"target_id"`
		RootEventID int64  `json:"root_event_id"`
		Title       string `json:"title,omitempty"`
		CreatedAt   string `json:"created_at"`
	}

	toEventItem := func(e db.ZdxEvent) EventItem {
		item := EventItem{
			ID:          e.ID,
			TargetType:  e.TargetType,
			TargetID:    e.TargetID,
			EventType:   e.EventType,
			Author:      e.Author,
			AuthorKind:  e.AuthorKind,
			SummaryJson: json.RawMessage(e.SummaryJson),
			DetailJson:  json.RawMessage(e.DetailJson),
			CreatedAt:   fmtTS(e.CreatedAt),
		}
		if e.ThreadID.Valid {
			item.ThreadID = &e.ThreadID.Int64
		}
		if len(e.AgentProcessResult) > 0 {
			item.AgentProcessResult = json.RawMessage(e.AgentProcessResult)
		}
		return item
	}

	toThreadItem := func(t db.ZdxEventThread) ThreadItem {
		item := ThreadItem{
			ID:          t.ID,
			TargetType:  t.TargetType,
			TargetID:    t.TargetID,
			RootEventID: t.RootEventID,
			CreatedAt:   fmtTS(t.CreatedAt),
		}
		if t.Title.Valid {
			item.Title = t.Title.String
		}
		return item
	}

	resolveAuthor := func(ctx context.Context) string {
		if uid := ctxUserIDVal(ctx); uid != 0 {
			if u, err := h.Q.GetUserByID(ctx, uid); err == nil {
				return u.Email
			}
		}
		return ""
	}

	huma.Register(api, huma.Operation{OperationID: "list-events", Method: http.MethodGet, Path: "/api/events"},
		func(ctx context.Context, in *struct {
			Slug       string `query:"slug" required:"true"`
			TargetType string `query:"target_type" required:"true"`
			TargetID   string `query:"target_id" required:"true"`
			ThreadID   int64  `query:"thread_id"`
		}) (*struct {
			Body struct {
				Events []EventItem `json:"events"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			var rows []db.ZdxEvent
			if in.ThreadID > 0 {
				rows, err = h.Q.ListEventsByThread(ctx, db.ListEventsByThreadParams{
					ProjectID:  p.ID,
					TargetType: in.TargetType,
					TargetID:   in.TargetID,
					ThreadID:   pgtype.Int8{Int64: in.ThreadID, Valid: true},
				})
			} else {
				rows, err = h.Q.ListEventsByTarget(ctx, db.ListEventsByTargetParams{
					ProjectID:  p.ID,
					TargetType: in.TargetType,
					TargetID:   in.TargetID,
				})
			}
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]EventItem, len(rows))
			for i, r := range rows {
				out[i] = toEventItem(r)
			}
			return &struct {
				Body struct {
					Events []EventItem `json:"events"`
				}
			}{Body: struct {
				Events []EventItem `json:"events"`
			}{Events: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "add-event-comment", Method: http.MethodPost, Path: "/api/events/comment"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug        string          `json:"slug"`
				TargetType  string          `json:"target_type"`
				TargetID    string          `json:"target_id"`
				EventType   string          `json:"event_type,omitempty"`
				AuthorKind  string          `json:"author_kind,omitempty"`
				SummaryJson json.RawMessage `json:"summary_json,omitempty"`
				DetailJson  json.RawMessage `json:"detail_json,omitempty"`
			}
		}) (*struct{ Body EventItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			eventType := in.Body.EventType
			if eventType == "" {
				eventType = "comment"
			}
			authorKind := in.Body.AuthorKind
			if authorKind == "" {
				authorKind = "user"
			}
			summary := []byte(in.Body.SummaryJson)
			if len(summary) == 0 {
				summary = []byte("{}")
			}
			detail := []byte(in.Body.DetailJson)
			if len(detail) == 0 {
				detail = []byte("{}")
			}
			e, err := h.Q.InsertEvent(ctx, db.InsertEventParams{
				ProjectID:   p.ID,
				TargetType:  in.Body.TargetType,
				TargetID:    in.Body.TargetID,
				ThreadID:    pgtype.Int8{Valid: false},
				EventType:   eventType,
				Author:      resolveAuthor(ctx),
				AuthorKind:  authorKind,
				SummaryJson: summary,
				DetailJson:  detail,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if _, err := h.Q.UpsertStream(ctx, db.UpsertStreamParams{
				ProjectID:       p.ID,
				TargetType:      in.Body.TargetType,
				TargetID:        in.Body.TargetID,
				LastEvaluatedAt: pgtype.Timestamptz{Valid: false},
				LastEvaluatedBy: pgtype.Text{Valid: false},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body EventItem }{Body: toEventItem(e)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reply-to-event", Method: http.MethodPost, Path: "/api/events/{id}/reply"},
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body struct {
				Slug        string          `json:"slug"`
				EventType   string          `json:"event_type,omitempty"`
				AuthorKind  string          `json:"author_kind,omitempty"`
				SummaryJson json.RawMessage `json:"summary_json,omitempty"`
				DetailJson  json.RawMessage `json:"detail_json,omitempty"`
			}
		}) (*struct{ Body EventItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			parent, err := h.Q.GetEventByID(ctx, in.ID)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "event not found")
			}
			if parent.ProjectID != p.ID {
				return nil, apiErr(http.StatusNotFound, "event not found")
			}
			thread, err := h.Q.GetThreadByRoot(ctx, in.ID)
			if err != nil {
				thread, err = h.Q.CreateThread(ctx, db.CreateThreadParams{
					ProjectID:   p.ID,
					TargetType:  parent.TargetType,
					TargetID:    parent.TargetID,
					RootEventID: in.ID,
					Title:       pgtype.Text{Valid: false},
				})
				if err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			eventType := in.Body.EventType
			if eventType == "" {
				eventType = "comment"
			}
			authorKind := in.Body.AuthorKind
			if authorKind == "" {
				authorKind = "user"
			}
			summary := []byte(in.Body.SummaryJson)
			if len(summary) == 0 {
				summary = []byte("{}")
			}
			detail := []byte(in.Body.DetailJson)
			if len(detail) == 0 {
				detail = []byte("{}")
			}
			e, err := h.Q.InsertEvent(ctx, db.InsertEventParams{
				ProjectID:   p.ID,
				TargetType:  parent.TargetType,
				TargetID:    parent.TargetID,
				ThreadID:    pgtype.Int8{Int64: thread.ID, Valid: true},
				EventType:   eventType,
				Author:      resolveAuthor(ctx),
				AuthorKind:  authorKind,
				SummaryJson: summary,
				DetailJson:  detail,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			if _, err := h.Q.UpsertStream(ctx, db.UpsertStreamParams{
				ProjectID:       p.ID,
				TargetType:      parent.TargetType,
				TargetID:        parent.TargetID,
				LastEvaluatedAt: pgtype.Timestamptz{Valid: false},
				LastEvaluatedBy: pgtype.Text{Valid: false},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body EventItem }{Body: toEventItem(e)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-event-verdict", Method: http.MethodPost, Path: "/api/events/{id}/verdict"},
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body struct {
				Slug               string          `json:"slug"`
				AgentProcessResult json.RawMessage `json:"agent_process_result"`
			}
		}) (*struct{ Body EventItem }, error) {
			p, err := getProject(ctx, h.Q, in.Body.Slug)
			if err != nil {
				return nil, err
			}
			existing, err := h.Q.GetEventByID(ctx, in.ID)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "event not found")
			}
			if existing.ProjectID != p.ID {
				return nil, apiErr(http.StatusNotFound, "event not found")
			}
			result := []byte(in.Body.AgentProcessResult)
			if len(result) == 0 {
				return nil, apiErr(http.StatusBadRequest, "agent_process_result is required")
			}
			e, err := h.Q.SetEventVerdict(ctx, db.SetEventVerdictParams{
				ID:                 in.ID,
				AgentProcessResult: result,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			evaluator := resolveAuthor(ctx)
			if _, err := h.Q.UpsertStream(ctx, db.UpsertStreamParams{
				ProjectID:       p.ID,
				TargetType:      existing.TargetType,
				TargetID:        existing.TargetID,
				LastEvaluatedAt: pgtype.Timestamptz{Time: time.Now(), Valid: true},
				LastEvaluatedBy: pgtype.Text{String: evaluator, Valid: evaluator != ""},
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body EventItem }{Body: toEventItem(e)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "set-event-thread-title", Method: http.MethodPatch, Path: "/api/events/threads/{id}/title"},
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body struct {
				Slug  string `json:"slug"`
				Title string `json:"title"`
			}
		}) (*struct{ Body ThreadItem }, error) {
			if _, err := getProject(ctx, h.Q, in.Body.Slug); err != nil {
				return nil, err
			}
			t, err := h.Q.SetThreadTitle(ctx, db.SetThreadTitleParams{
				ID:    in.ID,
				Title: pgtype.Text{String: in.Body.Title, Valid: in.Body.Title != ""},
			})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "thread not found")
			}
			return &struct{ Body ThreadItem }{Body: toThreadItem(t)}, nil
		})
}
