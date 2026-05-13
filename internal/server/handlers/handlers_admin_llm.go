package handlers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
	"github.com/iodesystems/zdx-go/internal/llm"
)

// LLMConfigBody is the JSON shape of a single LLM configuration on the wire.
// Empty strings in embedding_model / model_low / model_medium / model_high /
// model_xhigh / model_max are treated as "not set" (DB NULL). On GET, api_key
// is omitted; has_api_key indicates whether a key is stored server-side.
type LLMConfigBody struct {
	ID             int64  `json:"id,omitempty"`
	Name           string `json:"name"`
	Priority       int32  `json:"priority"`
	Type           string `json:"type"`
	URL            string `json:"url"`
	EmbeddingModel string `json:"embedding_model"`
	APIKey         string `json:"api_key,omitempty"`
	HasAPIKey      bool   `json:"has_api_key,omitempty"`
	AgentType      string `json:"agent_type"` // openai | local | claude
	ModelLow       string `json:"model_low"`
	ModelMedium    string `json:"model_medium"`
	ModelHigh      string `json:"model_high"`
	// ModelXHigh / ModelMax were added in IS-1175 and are optional on the wire
	// so existing clients that don't yet send them continue to work; absent
	// values resolve via the IS-1176 fall-through-to-high path.
	ModelXHigh     string `json:"model_xhigh,omitempty"`
	ModelMax       string `json:"model_max,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

func textOrNull(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{}
	}
	return pgtype.Text{String: s, Valid: true}
}

func textString(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

type llmConfigRow interface {
	getID() int64
	getName() string
	getPriority() int32
	getType() string
	getURL() string
	getEmbeddingModel() pgtype.Text
	getAPIKey() string
	getAgentType() string
	getModelLow() pgtype.Text
	getModelMedium() pgtype.Text
	getModelHigh() pgtype.Text
	getModelXHigh() pgtype.Text
	getModelMax() pgtype.Text
	getTimeoutSeconds() int32
}

func toBody(r llmConfigRow, includeKey bool) LLMConfigBody {
	b := LLMConfigBody{
		ID:             r.getID(),
		Name:           r.getName(),
		Priority:       r.getPriority(),
		Type:           r.getType(),
		URL:            r.getURL(),
		EmbeddingModel: textString(r.getEmbeddingModel()),
		AgentType:      r.getAgentType(),
		ModelLow:       textString(r.getModelLow()),
		ModelMedium:    textString(r.getModelMedium()),
		ModelHigh:      textString(r.getModelHigh()),
		ModelXHigh:     textString(r.getModelXHigh()),
		ModelMax:       textString(r.getModelMax()),
		TimeoutSeconds: int(r.getTimeoutSeconds()),
		HasAPIKey:      r.getAPIKey() != "",
	}
	if includeKey {
		b.APIKey = r.getAPIKey()
	}
	return b
}

// Shim each *Row type to llmConfigRow without generics.
type listRow db.ListLLMConfigsRow

func (r listRow) getID() int64                   { return r.ID }
func (r listRow) getName() string                { return r.Name }
func (r listRow) getPriority() int32             { return r.Priority }
func (r listRow) getType() string                { return r.Type }
func (r listRow) getURL() string                 { return r.Url }
func (r listRow) getEmbeddingModel() pgtype.Text { return r.EmbeddingModel }
func (r listRow) getAPIKey() string              { return r.ApiKey }
func (r listRow) getAgentType() string           { return r.AgentType }
func (r listRow) getModelLow() pgtype.Text       { return r.ModelLow }
func (r listRow) getModelMedium() pgtype.Text    { return r.ModelMedium }
func (r listRow) getModelHigh() pgtype.Text      { return r.ModelHigh }
func (r listRow) getModelXHigh() pgtype.Text     { return r.ModelXhigh }
func (r listRow) getModelMax() pgtype.Text       { return r.ModelMax }
func (r listRow) getTimeoutSeconds() int32       { return r.TimeoutSeconds }

type getRow db.GetLLMConfigByIDRow

func (r getRow) getID() int64                   { return r.ID }
func (r getRow) getName() string                { return r.Name }
func (r getRow) getPriority() int32             { return r.Priority }
func (r getRow) getType() string                { return r.Type }
func (r getRow) getURL() string                 { return r.Url }
func (r getRow) getEmbeddingModel() pgtype.Text { return r.EmbeddingModel }
func (r getRow) getAPIKey() string              { return r.ApiKey }
func (r getRow) getAgentType() string           { return r.AgentType }
func (r getRow) getModelLow() pgtype.Text       { return r.ModelLow }
func (r getRow) getModelMedium() pgtype.Text    { return r.ModelMedium }
func (r getRow) getModelHigh() pgtype.Text      { return r.ModelHigh }
func (r getRow) getModelXHigh() pgtype.Text     { return r.ModelXhigh }
func (r getRow) getModelMax() pgtype.Text       { return r.ModelMax }
func (r getRow) getTimeoutSeconds() int32       { return r.TimeoutSeconds }

type createRow db.CreateLLMConfigRow

func (r createRow) getID() int64                   { return r.ID }
func (r createRow) getName() string                { return r.Name }
func (r createRow) getPriority() int32             { return r.Priority }
func (r createRow) getType() string                { return r.Type }
func (r createRow) getURL() string                 { return r.Url }
func (r createRow) getEmbeddingModel() pgtype.Text { return r.EmbeddingModel }
func (r createRow) getAPIKey() string              { return r.ApiKey }
func (r createRow) getAgentType() string           { return r.AgentType }
func (r createRow) getModelLow() pgtype.Text       { return r.ModelLow }
func (r createRow) getModelMedium() pgtype.Text    { return r.ModelMedium }
func (r createRow) getModelHigh() pgtype.Text      { return r.ModelHigh }
func (r createRow) getModelXHigh() pgtype.Text     { return r.ModelXhigh }
func (r createRow) getModelMax() pgtype.Text       { return r.ModelMax }
func (r createRow) getTimeoutSeconds() int32       { return r.TimeoutSeconds }

type updateRow db.UpdateLLMConfigRow

func (r updateRow) getID() int64                   { return r.ID }
func (r updateRow) getName() string                { return r.Name }
func (r updateRow) getPriority() int32             { return r.Priority }
func (r updateRow) getType() string                { return r.Type }
func (r updateRow) getURL() string                 { return r.Url }
func (r updateRow) getEmbeddingModel() pgtype.Text { return r.EmbeddingModel }
func (r updateRow) getAPIKey() string              { return r.ApiKey }
func (r updateRow) getAgentType() string           { return r.AgentType }
func (r updateRow) getModelLow() pgtype.Text       { return r.ModelLow }
func (r updateRow) getModelMedium() pgtype.Text    { return r.ModelMedium }
func (r updateRow) getModelHigh() pgtype.Text      { return r.ModelHigh }
func (r updateRow) getModelXHigh() pgtype.Text     { return r.ModelXhigh }
func (r updateRow) getModelMax() pgtype.Text       { return r.ModelMax }
func (r updateRow) getTimeoutSeconds() int32       { return r.TimeoutSeconds }

func (h *Handler) registerAdminLLMConfigRoutes(api huma.API) {
	huma.Register(api, huma.Operation{OperationID: "list-llm-configs", Method: http.MethodGet, Path: "/api/admin/llm-configs"},
		func(ctx context.Context, _ *struct{}) (*struct {
			Body struct {
				Configs []LLMConfigBody `json:"configs"`
			}
		}, error) {
			if !h.Features.HasLLMConfig {
				return &struct {
					Body struct {
						Configs []LLMConfigBody `json:"configs"`
					}
				}{}, nil
			}
			rows, err := h.Q.ListLLMConfigs(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]LLMConfigBody, len(rows))
			for i, r := range rows {
				items[i] = toBody(listRow(r), false)
			}
			return &struct {
				Body struct {
					Configs []LLMConfigBody `json:"configs"`
				}
			}{Body: struct {
				Configs []LLMConfigBody `json:"configs"`
			}{Configs: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-llm-config", Method: http.MethodGet, Path: "/api/admin/llm-configs/{id}"},
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{ Body LLMConfigBody }, error) {
			if !h.Features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			row, err := h.Q.GetLLMConfigByID(ctx, in.ID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, apiErr(http.StatusNotFound, "llm config not found")
				}
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body LLMConfigBody }{Body: toBody(getRow(row), false)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "create-llm-config", Method: http.MethodPost, Path: "/api/admin/llm-configs"},
		func(ctx context.Context, in *struct{ Body LLMConfigBody }) (*struct{ Body LLMConfigBody }, error) {
			if !h.Features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			if err := validateAgentTypeEmbedding(in.Body); err != nil {
				return nil, apiErr(http.StatusBadRequest, err.Error())
			}
			agentType := in.Body.AgentType
			if agentType == "" {
				agentType = "openai"
			}
			ltype := in.Body.Type
			if ltype == "" {
				ltype = "openai"
			}
			name := in.Body.Name
			if name == "" {
				name = "default"
			}
			priority, err := h.Q.NextLLMConfigPriority(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			timeout := in.Body.TimeoutSeconds
			if timeout <= 0 {
				timeout = 600
			}
			row, err := h.Q.CreateLLMConfig(ctx, db.CreateLLMConfigParams{
				Name:           name,
				Priority:       priority,
				Type:           ltype,
				Url:            in.Body.URL,
				EmbeddingModel: textOrNull(in.Body.EmbeddingModel),
				ApiKey:         in.Body.APIKey,
				AgentType:      agentType,
				ModelLow:       textOrNull(in.Body.ModelLow),
				ModelMedium:    textOrNull(in.Body.ModelMedium),
				ModelHigh:      textOrNull(in.Body.ModelHigh),
				ModelXhigh:     textOrNull(in.Body.ModelXHigh),
				ModelMax:       textOrNull(in.Body.ModelMax),
				TimeoutSeconds: int32(timeout),
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.IngestRegistrar.ReloadEmbedder(ctx)
			go h.IngestRegistrar.ReindexAllIssues()
			return &struct{ Body LLMConfigBody }{Body: toBody(createRow(row), false)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "update-llm-config", Method: http.MethodPut, Path: "/api/admin/llm-configs/{id}"},
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body LLMConfigBody
		}) (*struct{ Body LLMConfigBody }, error) {
			if !h.Features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			if err := validateAgentTypeEmbedding(in.Body); err != nil {
				return nil, apiErr(http.StatusBadRequest, err.Error())
			}
			// Empty api_key means "keep existing" — fetch current value.
			apiKey := in.Body.APIKey
			if apiKey == "" {
				cur, err := h.Q.GetLLMConfigByID(ctx, in.ID)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return nil, apiErr(http.StatusNotFound, "llm config not found")
					}
					return nil, apiErr(500, err.Error())
				}
				apiKey = cur.ApiKey
			}
			agentType := in.Body.AgentType
			if agentType == "" {
				agentType = "openai"
			}
			ltype := in.Body.Type
			if ltype == "" {
				ltype = "openai"
			}
			name := in.Body.Name
			if name == "" {
				name = "default"
			}
			timeout := in.Body.TimeoutSeconds
			if timeout <= 0 {
				timeout = 600
			}
			row, err := h.Q.UpdateLLMConfig(ctx, db.UpdateLLMConfigParams{
				ID:             in.ID,
				Name:           name,
				Type:           ltype,
				Url:            in.Body.URL,
				EmbeddingModel: textOrNull(in.Body.EmbeddingModel),
				ApiKey:         apiKey,
				AgentType:      agentType,
				ModelLow:       textOrNull(in.Body.ModelLow),
				ModelMedium:    textOrNull(in.Body.ModelMedium),
				ModelHigh:      textOrNull(in.Body.ModelHigh),
				ModelXhigh:     textOrNull(in.Body.ModelXHigh),
				ModelMax:       textOrNull(in.Body.ModelMax),
				TimeoutSeconds: int32(timeout),
			})
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return nil, apiErr(http.StatusNotFound, "llm config not found")
				}
				return nil, apiErr(500, err.Error())
			}
			h.IngestRegistrar.ReloadEmbedder(ctx)
			go h.IngestRegistrar.ReindexAllIssues()
			return &struct{ Body LLMConfigBody }{Body: toBody(updateRow(row), false)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "delete-llm-config", Method: http.MethodDelete, Path: "/api/admin/llm-configs/{id}"},
		func(ctx context.Context, in *struct {
			ID int64 `path:"id"`
		}) (*struct{}, error) {
			if !h.Features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			if err := h.Q.DeleteLLMConfig(ctx, in.ID); err != nil {
				return nil, apiErr(500, err.Error())
			}
			h.IngestRegistrar.ReloadEmbedder(ctx)
			return &struct{}{}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "reorder-llm-configs", Method: http.MethodPost, Path: "/api/admin/llm-configs/reorder"},
		func(ctx context.Context, in *struct {
			Body struct {
				IDs []int64 `json:"ids"`
			}
		}) (*struct {
			Body struct {
				Configs []LLMConfigBody `json:"configs"`
			}
		}, error) {
			if !h.Features.HasLLMConfig {
				return nil, apiErr(http.StatusServiceUnavailable, "LLM config schema not yet applied — run: dx migrate up")
			}
			if h.Pool == nil {
				return nil, apiErr(500, "reorder requires direct pool access")
			}
			tx, err := h.Pool.Begin(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			defer tx.Rollback(ctx)

			// Defer the priority UNIQUE check to COMMIT so intermediate values
			// during the per-row updates don't trip the constraint.
			if _, err := tx.Exec(ctx, "SET CONSTRAINTS zdx_llm_configs_priority_uniq DEFERRED"); err != nil {
				return nil, apiErr(500, err.Error())
			}
			q := h.Q.WithTx(tx)
			for i, id := range in.Body.IDs {
				if err := q.UpdateLLMConfigPriority(ctx, db.UpdateLLMConfigPriorityParams{
					ID:       id,
					Priority: int32(i + 1),
				}); err != nil {
					return nil, apiErr(500, err.Error())
				}
			}
			if err := tx.Commit(ctx); err != nil {
				return nil, apiErr(500, err.Error())
			}

			rows, err := h.Q.ListLLMConfigs(ctx)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			items := make([]LLMConfigBody, len(rows))
			for i, r := range rows {
				items[i] = toBody(listRow(r), false)
			}
			return &struct {
				Body struct {
					Configs []LLMConfigBody `json:"configs"`
				}
			}{Body: struct {
				Configs []LLMConfigBody `json:"configs"`
			}{Configs: items}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-llm-models", Method: http.MethodGet, Path: "/api/admin/llm-configs/{id}/models"},
		func(ctx context.Context, in *struct {
			ID  int64  `path:"id"`
			URL string `query:"url"`
		}) (*struct {
			Body struct {
				Models []string `json:"models"`
			}
		}, error) {
			url := in.URL
			apiKey := ""
			ltype := "openai"
			if h.Features.HasLLMConfig && in.ID != 0 {
				cur, err := h.Q.GetLLMConfigByID(ctx, in.ID)
				if err == nil {
					if url == "" {
						url = cur.Url
					}
					apiKey = cur.ApiKey
					ltype = cur.Type
				}
			}
			if url == "" {
				return nil, apiErr(http.StatusBadRequest, "no LLM URL configured — set one in admin/llm or pass ?url=")
			}
			client := llm.New(llm.Config{Type: ltype, URL: url, APIKey: apiKey})
			tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			models, err := client.ListModels(tctx)
			if err != nil {
				return nil, apiErr(http.StatusBadGateway, err.Error())
			}
			return &struct {
				Body struct {
					Models []string `json:"models"`
				}
			}{Body: struct {
				Models []string `json:"models"`
			}{Models: models}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "test-llm-config", Method: http.MethodPost, Path: "/api/admin/llm-configs/{id}/test"},
		func(ctx context.Context, in *struct {
			ID   int64 `path:"id"`
			Body LLMConfigBody
		}) (*struct {
			Body struct {
				OK      bool   `json:"ok"`
				Message string `json:"message"`
			}
		}, error) {
			apiKey := in.Body.APIKey
			model := in.Body.EmbeddingModel
			url := in.Body.URL
			ltype := in.Body.Type
			if h.Features.HasLLMConfig && in.ID != 0 {
				if cur, err := h.Q.GetLLMConfigByID(ctx, in.ID); err == nil {
					if apiKey == "" {
						apiKey = cur.ApiKey
					}
					if model == "" {
						model = textString(cur.EmbeddingModel)
					}
					if url == "" {
						url = cur.Url
					}
					if ltype == "" {
						ltype = cur.Type
					}
				}
			}
			resp := &struct {
				Body struct {
					OK      bool   `json:"ok"`
					Message string `json:"message"`
				}
			}{}
			if model == "" {
				resp.Body.OK = false
				resp.Body.Message = "no embedding_model set for this config"
				return resp, nil
			}
			client := llm.New(llm.Config{
				Type:   ltype,
				URL:    url,
				Model:  model,
				APIKey: apiKey,
			})
			tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			vec, err := client.Embed(tctx, "test")
			if err != nil {
				resp.Body.OK = false
				resp.Body.Message = err.Error()
				return resp, nil
			}
			resp.Body.OK = true
			resp.Body.Message = fmt.Sprintf("ok (vector dim=%d)", len(vec))
			return resp, nil
		})
}

// validateAgentTypeEmbedding enforces the schema CHECK at the API boundary
// so callers get a 400 instead of a raw 23514 constraint-violation error.
func validateAgentTypeEmbedding(b LLMConfigBody) error {
	if b.AgentType == "claude" && b.EmbeddingModel != "" {
		return errors.New("embedding_model must be empty when agent_type='claude' — Claude handles embeddings differently")
	}
	return nil
}
