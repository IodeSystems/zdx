package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/todoclaim"
)

// testFixReasons names the incomplete-report reasons that auto-promote to a
// high-priority test-fix issue (TK-1617). Other reasons keep accumulating in
// zdx_todo_incomplete_reports without a downstream side effect at write time.
var testFixReasons = map[string]bool{
	"preexisting_test_failure": true,
	"flaky_test":               true,
}

// autoPromoteTestFixEnabled honors ZDX_AUTO_PROMOTE_TEST_FIX (default on); set
// to "off"/"0"/"false" to skip the side effect for safe rollout.
func autoPromoteTestFixEnabled() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("ZDX_AUTO_PROMOTE_TEST_FIX"))) {
	case "off", "0", "false", "no":
		return false
	}
	return true
}

type TodoSessionItem struct {
	ID        int64  `json:"id"`
	SessionID string `json:"session_id"`
	IssueID   string `json:"issue_id,omitempty"`
	Title     string `json:"title,omitempty"`
	Alias     string `json:"alias,omitempty"`
	Header    string `json:"header,omitempty"`
	Summary   string `json:"summary,omitempty"`
	Status    string `json:"status,omitempty"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ClosedAt  string `json:"closed_at,omitempty"`
}

type TodoDetailBody struct {
	Todo         TodoItem          `json:"todo"`
	Reservations []ReservationItem `json:"reservations"`
	Sessions     []TodoSessionItem `json:"sessions"`
}

func todoInstructions(kind string) string {
	switch kind {
	case "product:triage":
		return `Triage checklist:
1. Verify independently — reproduce or read the relevant code before accepting the report.
2. Dup-check: dx issue list; close duplicates with --reason=duplicate --duplicate-of=IS-X.
3. Rewrite prescriptively: title = intended outcome; context = should/did/direction.
4. Apply: dx todo owner triage IS-N --title=... --context=... --type=<ops|impl|ask|tracker> --priority=<1-4> --focus=<FO-N> --goal=<G-N>
   type guide:
     ops     = one-time verifiable action (demo/test plan required)
     impl    = durable code change (resolution link required to close)
     ask     = investigation/research/justification (no test plan; may spawn follow-up issues)
     tracker = umbrella issue (closed by its children; agent queue skips it)
If the issue is too vague, file clarification questions:
  dx question add --target-type=issue --target-id=IS-N --context="<question>" --choices="opt1,opt2,..."`
	case "add":
		return `Decompose this issue into dev tasks:
  dx todo tech add --issue=<issue-ref> --title=<outcome> --text=<plan> --reason=<why> --test-plan=<verification>
Add one task per logical step. Each task should be independently implementable and verifiable.`
	case "dev":
		return `Implement the task, then close it:
  dx todo dev done <task-id> --test-plan="<what was verified>" --file <path>[:start[-end]]
For impl issues, --file is required so the issue's code-ref trail is populated.
Claim before starting: dx todo take --agent-id=<id>`
	case "closable":
		return `All tasks are done — close the issue:
  dx issue close <issue-ref> --reason=done`
	case "close:tracker":
		return `All dependency issues are closed — close the tracker:
  dx issue close <issue-ref> --reason=done`
	case "clarify":
		return `A blocker question is pending. Answer it, then re-run dx todo queue:
  dx question answer <QA-N> --answer="<answer>"`
	case "read:comments":
		return `Unread comments need a response:
1. Read the comments and understand the context (question, feedback, or decision).
2. Reply: dx comment add <type> <id> --body="<reply>"
3. The queue marks comments read automatically after showing them.`
	case "respond:stale":
		return `A comment has gone unanswered for >24 hours. Reply or acknowledge:
  dx comment add <type> <id> --body="<reply>"`
	case "answer":
		return `An unanswered QA question needs a response:
  dx qa answer <QA-N> --answer="<answer>"`
	case "review":
		return `Review the completed task — verify it meets the test plan, then approve or reopen.`
	case "review:stale":
		return `This task was created but never claimed. Verify whether the work is still needed:
1. Read the referenced files to check if the work is already done or superseded.
2. If already implemented: dx todo dev done <task-id>
3. If still needed: proceed with implementation.`
	default:
		return ""
	}
}

var validIncompleteReasons = map[string]bool{
	"capability_gap":           true,
	"ambiguous_spec":           true,
	"external_dep":             true,
	"needs_decision":           true,
	"permission_denied":        true,
	"environment_broken":       true,
	"preexisting_test_failure": true,
	"flaky_test":               true,
}

func incompleteEvidenceFingerprint(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([][2]string, len(keys))
	for i, k := range keys {
		pairs[i] = [2]string{k, m[k]}
	}
	b, _ := json.Marshal(pairs)
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func (h *Handler) todoIncompleteStore() TodoIncompleteStore {
	if h.TodoIncompleteStore != nil {
		return h.TodoIncompleteStore
	}
	if h.Q != nil {
		return h.Q
	}
	return nil
}

type IncompleteReportItem struct {
	ID                  int64  `json:"id"`
	ProjectID           int32  `json:"project_id"`
	TodoID              int32  `json:"todo_id"`
	Reason              string `json:"reason"`
	Explanation         string `json:"explanation"`
	SuggestedNext       string `json:"suggested_next,omitempty"`
	Evidence            string `json:"evidence,omitempty"`
	EvidenceFingerprint string `json:"evidence_fingerprint"`
	AgentID             string `json:"agent_id,omitempty"`
	CreatedAt           string `json:"created_at"`
}

func (h *Handler) registerTodoRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "get-todo-detail", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/todos/{key}"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Key  string `path:"key"`
		}) (*struct{ Body TodoDetailBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			t, err := h.Q.GetTodoByKey(ctx, db.GetTodoByKeyParams{ProjectID: p.ID, Key: in.Key})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "todo not found: "+in.Key)
			}
			todo := TodoItem{
				ID:               t.ID,
				Text:             t.Text,
				Title:            t.Title,
				Description:      t.Description,
				Key:              t.Key,
				Persona:          t.Persona,
				Priority:         t.Priority,
				Status:           t.Status,
				TargetType:       t.TargetType,
				TargetID:         t.TargetID,
				Kind:             t.Kind,
				IssueRef:         t.IssueRef,
				Blocked:          t.Blocked,
				BlockedReason:    t.BlockedReason,
				CycleCount:       t.CycleCount,
				ReferenceIssueID: t.ReferenceIssueID,
				Instructions:     todoInstructions(t.Kind),
				ClaimContract:    todoclaim.ForKind(t.Kind).Name,
				ClaimedBy:        t.ClaimedBy,
				ClaimedAt:        fmtTS(t.ClaimedAt),
				CreatedAt:        fmtTS(t.CreatedAt),
				ResolvedAt:       fmtTS(t.ResolvedAt),
				Source:           t.Source,
			}

			resRows, err := h.Q.ListReservationsByTodoKey(ctx, db.ListReservationsByTodoKeyParams{
				ProjectID: p.ID,
				Key:       in.Key,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			reservations := make([]ReservationItem, len(resRows))
			for i, r := range resRows {
				item := ReservationItem{
					ID:             r.ID,
					TargetType:     r.TargetType,
					TargetID:       r.TargetID,
					ClaimedBy:      r.ClaimedBy,
					ClaimedAt:      fmtTS(r.ClaimedAt),
					ReleasedAt:     fmtTS(r.ReleasedAt),
					LeaseExpiresAt: fmtTS(r.LeaseExpiresAt),
				}
				if r.SessionID.Valid {
					item.SessionID = r.SessionID.Int64
					item.SessionStatus = r.SessionStatus.String
					item.SessionClosedAt = fmtTS(r.SessionClosedAt)
					item.SessionHeader = r.SessionHeader.String
					item.SessionAlias = r.SessionAlias.String
				}
				reservations[i] = item
			}

			sessRows, err := h.Q.ListClaudeSessionsByTodoID(ctx, db.ListClaudeSessionsByTodoIDParams{
				ProjectID: p.ID,
				TodoID:    pgtype.Int4{Int32: t.ID, Valid: true},
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			sessions := make([]TodoSessionItem, len(sessRows))
			for i, s := range sessRows {
				sessions[i] = TodoSessionItem{
					ID:        s.ID,
					SessionID: s.SessionID,
					IssueID:   s.IssueID,
					Title:     s.Title,
					Alias:     s.Alias,
					Header:    s.Header,
					Summary:   s.Summary,
					Status:    s.Status,
					CreatedAt: fmtTS(s.CreatedAt),
					UpdatedAt: fmtTS(s.UpdatedAt),
					ClosedAt:  fmtTS(s.ClosedAt),
				}
			}

			return &struct{ Body TodoDetailBody }{Body: TodoDetailBody{
				Todo:         todo,
				Reservations: reservations,
				Sessions:     sessions,
			}}, nil
		})

	// Operator-driven priority push — bumps a todo to the front of the claim
	// queue by writing a smaller `priority` integer. UpsertTodo's LEAST()
	// clause makes pushes survive re-evaluate, so this isn't a one-shot.
	// Per docs/plan.md (GAPD phase 3, mark-as-priority element).
	huma.Register(api, huma.Operation{OperationID: "set-todo-priority", Method: http.MethodPut, Path: "/api/dx/projects/{slug}/todos/{key}/priority"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Key  string `path:"key"`
			Body struct {
				Priority int32 `json:"priority" required:"true"`
			}
		}) (*struct {
			Body struct {
				Priority int32 `json:"priority"`
			}
		}, error) {
			if in.Body.Priority < 0 {
				return nil, apiErr(http.StatusBadRequest, "priority must be >= 0")
			}
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if err := h.Q.SetTodoPriority(ctx, db.SetTodoPriorityParams{
				Priority:  in.Body.Priority,
				ProjectID: p.ID,
				Key:       in.Key,
			}); err != nil {
				return nil, apiErr(500, err.Error())
			}
			traceEvent(ctx, h.Q, p.ID, "todo.priority_set", "todo_key", in.Key, "priority", in.Body.Priority)
			return &struct {
				Body struct {
					Priority int32 `json:"priority"`
				}
			}{Body: struct {
				Priority int32 `json:"priority"`
			}{Priority: in.Body.Priority}}, nil
		})

	type postIncompleteReportInput struct {
		Slug string `path:"slug"`
		Key  string `path:"key"`
		Body struct {
			Reason        string            `json:"reason"`
			Explanation   string            `json:"explanation"`
			SuggestedNext string            `json:"suggested_next,omitempty"`
			Evidence      map[string]string `json:"evidence,omitempty"`
			AgentID       string            `json:"agent_id,omitempty"`
		}
	}

	huma.Register(api, huma.Operation{OperationID: "post-todo-incomplete-report", Method: http.MethodPost, Path: "/api/dx/projects/{slug}/todos/{key}/incomplete-reports"},
		func(ctx context.Context, in *postIncompleteReportInput) (*struct{ Body IncompleteReportItem }, error) {
			if !validIncompleteReasons[in.Body.Reason] {
				return nil, apiErr(http.StatusBadRequest, "reason must be one of: capability_gap, ambiguous_spec, external_dep, needs_decision, permission_denied, environment_broken, preexisting_test_failure, flaky_test")
			}
			store := h.todoIncompleteStore()
			p, err := store.GetProjectBySlug(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			t, err := resolveIncompleteTodo(ctx, store, p.ID, in.Key)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "todo not found: "+in.Key)
			}

			fp := incompleteEvidenceFingerprint(in.Body.Evidence)

			var suggestedNextJSON []byte
			if in.Body.SuggestedNext != "" {
				suggestedNextJSON, _ = json.Marshal(map[string]string{"text": in.Body.SuggestedNext})
			} else {
				suggestedNextJSON = []byte("null")
			}
			var evidenceJSON []byte
			if len(in.Body.Evidence) > 0 {
				evidenceJSON, _ = json.Marshal(in.Body.Evidence)
			} else {
				evidenceJSON = []byte("null")
			}

			row, err := store.AddTodoIncompleteReport(ctx, db.AddTodoIncompleteReportParams{
				ProjectID:           p.ID,
				TodoID:              t.ID,
				Reason:              in.Body.Reason,
				Explanation:         in.Body.Explanation,
				SuggestedNext:       suggestedNextJSON,
				Evidence:            evidenceJSON,
				EvidenceFingerprint: fp,
				AgentID:             in.Body.AgentID,
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}

			if testFixReasons[in.Body.Reason] && autoPromoteTestFixEnabled() {
				h.promoteTestFix(ctx, store, p.ID, t, in.Body.Reason, fp, in.Body.Explanation, in.Body.Evidence, in.Body.AgentID)
			}

			item := IncompleteReportItem{
				ID:                  row.ID,
				ProjectID:           row.ProjectID,
				TodoID:              row.TodoID,
				Reason:              row.Reason,
				Explanation:         row.Explanation,
				EvidenceFingerprint: row.EvidenceFingerprint,
				AgentID:             row.AgentID,
				CreatedAt:           fmtTS(row.CreatedAt),
			}
			if len(row.SuggestedNext) > 0 && string(row.SuggestedNext) != "null" {
				item.SuggestedNext = string(row.SuggestedNext)
			}
			if len(row.Evidence) > 0 && string(row.Evidence) != "null" {
				item.Evidence = string(row.Evidence)
			}
			return &struct{ Body IncompleteReportItem }{Body: item}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-todo-incomplete-reports", Method: http.MethodGet, Path: "/api/dx/projects/{slug}/todos/{key}/incomplete-reports"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Key  string `path:"key"`
		}) (*struct{ Body []IncompleteReportItem }, error) {
			store := h.todoIncompleteStore()
			p, err := store.GetProjectBySlug(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			t, err := resolveIncompleteTodo(ctx, store, p.ID, in.Key)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "todo not found: "+in.Key)
			}
			rows, err := store.GetTodoIncompleteReportsByTodo(ctx, t.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			items := make([]IncompleteReportItem, len(rows))
			for i, row := range rows {
				items[i] = IncompleteReportItem{
					ID:                  row.ID,
					ProjectID:           row.ProjectID,
					TodoID:              row.TodoID,
					Reason:              row.Reason,
					Explanation:         row.Explanation,
					EvidenceFingerprint: row.EvidenceFingerprint,
					AgentID:             row.AgentID,
					CreatedAt:           fmtTS(row.CreatedAt),
				}
				if len(row.SuggestedNext) > 0 && string(row.SuggestedNext) != "null" {
					items[i].SuggestedNext = string(row.SuggestedNext)
				}
				if len(row.Evidence) > 0 && string(row.Evidence) != "null" {
					items[i].Evidence = string(row.Evidence)
				}
			}
			return &struct{ Body []IncompleteReportItem }{Body: items}, nil
		})

	type IncompleteReportGroup struct {
		Reason              string   `json:"reason"`
		EvidenceFingerprint string   `json:"evidence_fingerprint"`
		TotalCount          int64    `json:"total_count"`
		AffectedTodoIDs     []int32  `json:"affected_todo_ids"`
		AffectedTodoKeys    []string `json:"affected_todo_keys"`
		LastSeen            string   `json:"last_seen"`
		SuggestedNext       string   `json:"suggested_next,omitempty"`
	}

	huma.Register(api, huma.Operation{OperationID: "list-incomplete-reports", Method: http.MethodGet, Path: "/api/dx/incomplete-reports"},
		func(ctx context.Context, in *struct {
			Slug   string `query:"slug"`
			Reason string `query:"reason"`
		}) (*struct {
			Body struct{ Groups []IncompleteReportGroup }
		}, error) {
			store := h.todoIncompleteStore()
			p, err := store.GetProjectBySlug(ctx, in.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Slug)
			}
			var reasonParam pgtype.Text
			if in.Reason != "" {
				reasonParam = pgtype.Text{String: in.Reason, Valid: true}
			}
			rows, err := store.AggregateTodoIncompleteReports(ctx, db.AggregateTodoIncompleteReportsParams{
				ProjectID: p.ID,
				Reason:    reasonParam,
			})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			groups := make([]IncompleteReportGroup, len(rows))
			for i, row := range rows {
				ids := row.AffectedTodoIds
				if ids == nil {
					ids = []int32{}
				}
				keys := row.AffectedTodoKeys
				if keys == nil {
					keys = []string{}
				}
				groups[i] = IncompleteReportGroup{
					Reason:              row.Reason,
					EvidenceFingerprint: row.EvidenceFingerprint,
					TotalCount:          row.TotalCount,
					AffectedTodoIDs:     ids,
					AffectedTodoKeys:    keys,
					LastSeen:            fmtTSAny(row.LastSeen),
				}
				if len(row.SuggestedNext) > 0 && string(row.SuggestedNext) != "null" {
					groups[i].SuggestedNext = string(row.SuggestedNext)
				}
			}
			return &struct {
				Body struct{ Groups []IncompleteReportGroup }
			}{
				Body: struct{ Groups []IncompleteReportGroup }{Groups: groups},
			}, nil
		})

	type AppliedSideEffect struct {
		Reason              string `json:"reason"`
		EvidenceFingerprint string `json:"evidence_fingerprint"`
		ActionType          string `json:"action_type"`
		Detail              string `json:"detail"`
	}

	huma.Register(api, huma.Operation{OperationID: "apply-incomplete-report-side-effects", Method: http.MethodPost, Path: "/api/dx/incomplete-reports/apply"},
		func(ctx context.Context, in *struct {
			Body struct {
				Slug string `json:"slug"`
			}
		}) (*struct {
			Body struct{ Applied []AppliedSideEffect }
		}, error) {
			store := h.todoIncompleteStore()
			p, err := store.GetProjectBySlug(ctx, in.Body.Slug)
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "project not found: "+in.Body.Slug)
			}
			rows, err := store.AggregateTodoIncompleteReports(ctx, db.AggregateTodoIncompleteReportsParams{ProjectID: p.ID})
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}

			applied := make([]AppliedSideEffect, 0)
			for _, group := range rows {
				if len(group.SuggestedNext) == 0 || string(group.SuggestedNext) == "null" {
					continue
				}
				var sn struct{ Text string }
				if err := json.Unmarshal(group.SuggestedNext, &sn); err != nil || sn.Text == "" {
					continue
				}
				text := sn.Text

				var actionType, detail string
				if rest, ok := strings.CutPrefix(text, "block on IS-"); ok && isAllDigits(rest) {
					actionType = "block_on"
					detail = "IS-" + rest
				} else if rest, ok := strings.CutPrefix(text, "ask user: "); ok {
					actionType = "ask_user"
					detail = rest
				} else if rest, ok := strings.CutPrefix(text, "file capability request: "); ok {
					actionType = "file_capability_request"
					detail = rest
				} else {
					continue
				}

				_, sideEffectErr := store.InsertSideEffectIfNew(ctx, db.InsertSideEffectIfNewParams{
					ProjectID:           p.ID,
					Reason:              group.Reason,
					EvidenceFingerprint: group.EvidenceFingerprint,
					ActionType:          actionType,
					Meta:                []byte("{}"),
				})
				if sideEffectErr != nil {
					if errors.Is(sideEffectErr, pgx.ErrNoRows) {
						continue
					}
					return nil, apiErr(http.StatusInternalServerError, sideEffectErr.Error())
				}

				switch actionType {
				case "block_on":
					if len(group.AffectedTodoIds) > 0 {
						todo, tErr := store.GetTodoByID(ctx, group.AffectedTodoIds[0])
						if tErr == nil && todo.TargetID != "" {
							blocker, bErr := store.GetIssueByAnyProject(ctx, detail)
							if bErr == nil {
								_ = store.AddIssueBlock(ctx, db.AddIssueBlockParams{
									IssueID:     todo.TargetID,
									BlockedByID: blocker.ID,
									Kind:        "sequencing",
								})
							}
						}
					}
				case "ask_user":
					_, _ = store.InsertQuestion(ctx, db.InsertQuestionParams{
						ProjectID: p.ID,
						Category:  "incomplete-report",
						Question:  detail,
					})
				case "file_capability_request":
					issueID, idErr := store.NextIssueID(ctx)
					if idErr == nil {
						_, _ = store.CreateIssue(ctx, db.CreateIssueParams{
							ID:        issueID,
							ProjectID: p.ID,
							Title:     detail,
							IssueType: "impl",
							Status:    "open",
							Priority:  "3",
							Component: "capability",
						})
					}
				}

				applied = append(applied, AppliedSideEffect{
					Reason:              group.Reason,
					EvidenceFingerprint: group.EvidenceFingerprint,
					ActionType:          actionType,
					Detail:              detail,
				})
			}
			return &struct {
				Body struct{ Applied []AppliedSideEffect }
			}{
				Body: struct{ Applied []AppliedSideEffect }{Applied: applied},
			}, nil
		})

	type TodoQueueHealthBody struct {
		TotalOpen             int64  `json:"total_open"`
		BlockedCount          int64  `json:"blocked_count"`
		UnblockedCount        int64  `json:"unblocked_count"`
		DominantBlockedReason string `json:"dominant_blocked_reason"`
	}

	huma.Register(api, huma.Operation{OperationID: "get-todo-queue-health", Method: http.MethodGet, Path: "/api/dx/todos/queue-health"},
		func(ctx context.Context, in *struct {
			Slug string `query:"slug" required:"true"`
		}) (*struct{ Body TodoQueueHealthBody }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			row, err := h.Q.GetTodoQueueHealth(ctx, p.ID)
			if err != nil {
				return nil, apiErr(http.StatusInternalServerError, err.Error())
			}
			return &struct{ Body TodoQueueHealthBody }{
				Body: TodoQueueHealthBody{
					TotalOpen:             row.TotalOpen,
					BlockedCount:          row.BlockedCount,
					UnblockedCount:        row.UnblockedCount,
					DominantBlockedReason: row.DominantBlockedReason,
				},
			}, nil
		})
}

// resolveIncompleteTodo accepts the {key} path param in either canonical
// form (e.g. "dev-IS-1096") or as a numeric todo ID (e.g. "862623"). The
// numeric path saves agents from discovering the canonical key form when
// they only know the ID from the claim seed (`Claimed todo 862623 ...`).
//
// sqlc generates GetTodoByKeyRow and GetTodoByIDRow from queries selecting
// the same columns in the same order, so the named struct types share an
// underlying type and Go permits the conversion. If one query's column
// list changes, the conversion stops compiling — the compiler catches the
// drift instead of silently mis-projecting a field.
func resolveIncompleteTodo(ctx context.Context, store TodoIncompleteStore, projectID int32, keyOrID string) (db.GetTodoByKeyRow, error) {
	if isAllDigits(keyOrID) {
		n, err := strconv.Atoi(keyOrID)
		if err != nil {
			return db.GetTodoByKeyRow{}, err
		}
		byID, err := store.GetTodoByID(ctx, int32(n))
		if err != nil {
			return db.GetTodoByKeyRow{}, err
		}
		if byID.ProjectID != projectID {
			return db.GetTodoByKeyRow{}, pgx.ErrNoRows
		}
		return db.GetTodoByKeyRow(byID), nil
	}
	return store.GetTodoByKey(ctx, db.GetTodoByKeyParams{ProjectID: projectID, Key: keyOrID})
}

func isAllDigits(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// promoteTestFix is the side effect for preexisting_test_failure / flaky_test
// reports (TK-1617). On the first report for a (reason, fingerprint) pair it
// files a priority-1 impl issue and records the mapping in zdx_test_fix_issues;
// subsequent reports append a comment to that issue. When the calling todo has
// an issue ref, a sequencing block edge is added so the source issue stays
// gated until the test-fix issue closes. Idempotent and best-effort: errors
// are swallowed because the parent report write has already succeeded.
func (h *Handler) promoteTestFix(ctx context.Context, store TodoIncompleteStore, projectID int32, todo db.GetTodoByKeyRow, reason, fingerprint, explanation string, evidence map[string]string, agentID string) {
	testName := evidence["test_name"]
	if testName == "" {
		testName = evidence["test"]
	}
	if testName == "" {
		testName = "(unknown test)"
	}

	existingID, err := store.GetTestFixIssue(ctx, db.GetTestFixIssueParams{
		ProjectID:           projectID,
		Reason:              reason,
		EvidenceFingerprint: fingerprint,
	})
	issueID := existingID
	created := false
	if errors.Is(err, pgx.ErrNoRows) {
		newID, idErr := store.NextIssueID(ctx)
		if idErr != nil {
			return
		}
		evJSON, _ := json.MarshalIndent(evidence, "", "  ")
		body := explanation
		if len(evidence) > 0 {
			body = strings.TrimSpace(body) + "\n\nEvidence:\n```json\n" + string(evJSON) + "\n```"
		}
		if _, cErr := store.CreateIssue(ctx, db.CreateIssueParams{
			ID:        newID,
			ProjectID: projectID,
			Title:     "Test failure: " + testName,
			Context:   body,
			Priority:  "1",
			Component: "test",
			IssueType: "impl",
			Status:    "open",
		}); cErr != nil {
			return
		}
		if iErr := store.InsertTestFixIssue(ctx, db.InsertTestFixIssueParams{
			ProjectID:           projectID,
			Reason:              reason,
			EvidenceFingerprint: fingerprint,
			IssueID:             newID,
		}); iErr != nil {
			return
		}
		issueID = newID
		created = true
	} else if err != nil {
		return
	}

	if !created {
		evJSON, _ := json.MarshalIndent(evidence, "", "  ")
		body := "Recurring " + reason + " report from todo " + todo.Key
		if agentID != "" {
			body += " (agent " + agentID + ")"
		}
		body += ":\n\n" + explanation
		if len(evidence) > 0 {
			body += "\n\nEvidence:\n```json\n" + string(evJSON) + "\n```"
		}
		_, _ = store.AddComment(ctx, db.AddCommentParams{
			ProjectID:   projectID,
			TargetType:  "issue",
			TargetID:    issueID,
			Author:      "system",
			AuthorAlias: "auto-promote",
			Body:        body,
		})
	}

	if todo.IssueRef != "" && issueID != "" && todo.IssueRef != issueID {
		_ = store.AddIssueBlock(ctx, db.AddIssueBlockParams{
			IssueID:     todo.IssueRef,
			BlockedByID: issueID,
			Kind:        "sequencing",
		})
	}
}
