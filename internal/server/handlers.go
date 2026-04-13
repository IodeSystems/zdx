package server

import (
	"net/http"
	"time"

	"github.com/iodesystems/zdx-go/internal/apitypes"
	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/jackc/pgx/v5/pgtype"
)

// ── projects ──────────────────────────────────────────────────────────────────

func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.q.ListProjects(r.Context())
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]apitypes.ProjectResp, len(projects))
	for i, p := range projects {
		out[i] = projectResp(p)
	}
	ok(w, out)
}

func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.CreateProjectRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	p, err := s.q.CreateProject(r.Context(), db.CreateProjectParams{Slug: req.Slug, Name: req.Name})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, projectResp(p))
}

// ── issues ────────────────────────────────────────────────────────────────────

func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	issues, err := s.q.ListIssues(r.Context(), project.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]apitypes.IssueResp, len(issues))
	for i, iss := range issues {
		out[i] = issueResp(iss)
	}
	ok(w, out)
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	id := r.URL.Query().Get("id")
	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	iss, err := s.q.GetIssue(r.Context(), db.GetIssueParams{ProjectID: project.ID, ID: id})
	if err != nil {
		fail(w, http.StatusNotFound, "issue not found")
		return
	}
	work, err := s.q.GetIssueWork(r.Context(), iss.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := issueResp(iss)
	resp.Work = make([]apitypes.IssueWorkResp, len(work))
	for i, wk := range work {
		resp.Work[i] = apitypes.IssueWorkResp{
			Agent:     wk.Agent,
			Note:      wk.Note,
			CreatedAt: fmtTS(wk.CreatedAt),
		}
	}
	ok(w, resp)
}

func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.CreateIssueRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	id, err := s.q.NextIssueID(r.Context(), project.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	iss, err := s.q.CreateIssue(r.Context(), db.CreateIssueParams{
		ID:        id,
		ProjectID: project.ID,
		Title:     req.Title,
		Context:   req.Context,
		Priority:  req.Priority,
		Component: req.Component,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, issueResp(iss))
}

func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.UpdateIssueRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	err = s.q.UpdateIssue(r.Context(), db.UpdateIssueParams{
		ProjectID: project.ID,
		ID:        req.ID,
		Title:     req.Title,
		Context:   req.Context,
		Priority:  req.Priority,
		BlockedBy: req.BlockedBy,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

func (s *Server) handleCloseIssue(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.CloseIssueRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	if err := s.q.CloseIssue(r.Context(), db.CloseIssueParams{ProjectID: project.ID, ID: req.ID}); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

func (s *Server) handleAppendIssueWork(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.AppendIssueWorkRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.q.AppendIssueWork(r.Context(), db.AppendIssueWorkParams{
		IssueID: req.ID,
		Agent:   req.Agent,
		Note:    req.Note,
	}); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

// ── tasks ─────────────────────────────────────────────────────────────────────

func (s *Server) handleListTasks(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	feature := r.URL.Query().Get("feature")
	issueID := r.URL.Query().Get("issue")

	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}

	var tasks []db.ZdxTask
	switch {
	case feature != "":
		tasks, err = s.q.ListTasksByFeature(r.Context(), db.ListTasksByFeatureParams{ProjectID: project.ID, Feature: feature})
	case issueID != "":
		tasks, err = s.q.ListTasksByIssue(r.Context(), db.ListTasksByIssueParams{ProjectID: project.ID, Issue: issueID})
	default:
		tasks, err = s.q.ListTasks(r.Context(), project.ID)
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]apitypes.TaskResp, len(tasks))
	for i, t := range tasks {
		out[i] = taskResp(t)
	}
	ok(w, out)
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.CreateTaskRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	id, err := s.q.NextTaskID(r.Context(), project.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	t, err := s.q.CreateTask(r.Context(), db.CreateTaskParams{
		ID:        id,
		ProjectID: project.ID,
		Text:      req.Text,
		Feature:   req.Feature,
		Issue:     req.Issue,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, taskResp(t))
}

func (s *Server) handleUpdateTaskStatus(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.UpdateTaskStatusRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.q.UpdateTaskStatus(r.Context(), db.UpdateTaskStatusParams{
		ID:     req.ID,
		Status: req.Status,
		Reason: req.Reason,
	}); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

func (s *Server) handleMarkTaskDone(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.MarkTaskDoneRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.q.MarkTaskDone(r.Context(), db.MarkTaskDoneParams{
		ID:       req.ID,
		TestPlan: req.TestPlan,
		TestRefs: req.TestRefs,
	}); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

func (s *Server) handleMarkTaskUndone(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.MarkTaskUndoneRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.q.MarkTaskUndone(r.Context(), req.ID); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

// ── features ──────────────────────────────────────────────────────────────────

func (s *Server) handleListFeatures(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	features, err := s.q.ListFeatures(r.Context(), project.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]apitypes.FeatureResp, len(features))
	for i, f := range features {
		out[i] = featureResp(f)
	}
	ok(w, out)
}

func (s *Server) handleGetFeature(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	name := r.URL.Query().Get("name")
	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	f, err := s.q.GetFeature(r.Context(), db.GetFeatureParams{ProjectID: project.ID, Name: name})
	if err != nil {
		fail(w, http.StatusNotFound, "feature not found")
		return
	}
	specs, err := s.q.ListSpecs(r.Context(), f.ID)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := featureResp(f)
	resp.Specs = make([]apitypes.SpecResp, len(specs))
	for i, sp := range specs {
		resp.Specs[i] = apitypes.SpecResp{
			ID:          sp.ID,
			Description: sp.Description,
			Kind:        sp.Kind,
		}
	}
	ok(w, resp)
}

func (s *Server) handleCreateFeature(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.CreateFeatureRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	f, err := s.q.UpsertFeature(r.Context(), db.UpsertFeatureParams{
		ProjectID:   project.ID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, featureResp(f))
}

func (s *Server) handleUpdateFeatureField(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.UpdateFeatureFieldRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	if err := s.q.UpdateFeatureField(r.Context(), db.UpdateFeatureFieldParams{
		ProjectID: project.ID,
		Name:      req.Feature,
		Field:     req.Field,
		Value:     req.Value,
	}); err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.OKResponse{OK: true})
}

func (s *Server) handleAddSpec(w http.ResponseWriter, r *http.Request) {
	req, err := bind[apitypes.AddSpecRequest](r)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.GetProjectBySlug(r.Context(), req.Slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}
	f, err := s.q.GetFeature(r.Context(), db.GetFeatureParams{ProjectID: project.ID, Name: req.Feature})
	if err != nil {
		fail(w, http.StatusNotFound, "feature not found")
		return
	}
	sp, err := s.q.AddSpec(r.Context(), db.AddSpecParams{
		FeatureID:   f.ID,
		Description: req.Description,
		Kind:        req.Kind,
	})
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, apitypes.SpecResp{
		ID:          sp.ID,
		Description: sp.Description,
		Kind:        sp.Kind,
	})
}

// ── solo ──────────────────────────────────────────────────────────────────────

func (s *Server) handleSolo(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	issueFilter := r.URL.Query().Get("issue")

	project, err := s.q.GetProjectBySlug(r.Context(), slug)
	if err != nil {
		fail(w, http.StatusNotFound, "project not found")
		return
	}

	item, err := computeSolo(r.Context(), s.q, project.ID, issueFilter)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	ok(w, item)
}

// ── response mappers ──────────────────────────────────────────────────────────

func fmtTS(ts pgtype.Timestamptz) string {
	if !ts.Valid {
		return ""
	}
	return ts.Time.UTC().Format(time.RFC3339)
}

func projectResp(p db.ZdxProject) apitypes.ProjectResp {
	return apitypes.ProjectResp{
		ID:        p.ID,
		Slug:      p.Slug,
		Name:      p.Name,
		CreatedAt: fmtTS(p.CreatedAt),
	}
}

func issueResp(i db.ZdxIssue) apitypes.IssueResp {
	return apitypes.IssueResp{
		ID:        i.ID,
		Title:     i.Title,
		Status:    i.Status,
		Priority:  i.Priority,
		Component: i.Component,
		Context:   i.Context,
		BlockedBy: i.BlockedBy,
		CreatedAt: fmtTS(i.CreatedAt),
	}
}

func taskResp(t db.ZdxTask) apitypes.TaskResp {
	resp := apitypes.TaskResp{
		ID:        t.ID,
		Text:      t.Text,
		Feature:   t.Feature,
		Status:    t.Status,
		Reason:    t.Reason,
		Issue:     t.Issue,
		Depends:   t.Depends,
		TestPlan:  t.TestPlan,
		TestRefs:  t.TestRefs,
		CreatedAt: fmtTS(t.CreatedAt),
	}
	if t.CompletedAt.Valid {
		s := fmtTS(t.CompletedAt)
		resp.CompletedAt = &s
	}
	return resp
}

func featureResp(f db.ZdxFeature) apitypes.FeatureResp {
	return apitypes.FeatureResp{
		ID:          f.ID,
		Name:        f.Name,
		Description: f.Description,
		What:        f.What,
		Why:         f.Why,
		DoneWhen:    f.DoneWhen,
		Component:   f.Component,
	}
}
