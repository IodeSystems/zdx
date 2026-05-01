package handlers

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/iodesystems/zdx-go/internal/db"
)

// ── Response types ────────────────────────────────────────────────────────

type AtlasNodeItem struct {
	ID          int64  `json:"id"`
	Kind        string `json:"kind"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type AtlasCoderefItem struct {
	ID        int64  `json:"id"`
	File      string `json:"file"`
	StartLine *int32 `json:"start_line,omitempty"`
	EndLine   *int32 `json:"end_line,omitempty"`
	Symbol    string `json:"symbol,omitempty"`
	Sha       string `json:"sha,omitempty"`
	CreatedAt string `json:"created_at"`
}

type AtlasChunkItem struct {
	ID           int64  `json:"id"`
	NodeID       int64  `json:"node_id"`
	CoderefID    *int64 `json:"coderef_id,omitempty"`
	Title        string `json:"title,omitempty"`
	Body         string `json:"body"`
	ShaAtWrite   string `json:"sha_at_write,omitempty"`
	VerifierKind string `json:"verifier_kind"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type AtlasStaleChunkItem struct {
	ID       int64  `json:"id"`
	NodeKind string `json:"node_kind"`
	NodeSlug string `json:"node_slug"`
	Title    string `json:"title,omitempty"`
	File     string `json:"file,omitempty"`
}

type AtlasEdgeItem struct {
	ID         int64  `json:"id"`
	FromNodeID int64  `json:"from_node_id"`
	ToNodeID   int64  `json:"to_node_id"`
	EdgeType   string `json:"edge_type"`
	Label      string `json:"label,omitempty"`
	Weight     int32  `json:"weight"`
	CreatedAt  string `json:"created_at"`
}

type AtlasEdgeFromItem struct {
	AtlasEdgeItem
	ToKind string `json:"to_kind"`
	ToSlug string `json:"to_slug"`
}

type AtlasNodeDetail struct {
	AtlasNodeItem
	Chunks []AtlasChunkItem    `json:"chunks"`
	Edges  []AtlasEdgeFromItem `json:"edges"`
}

// ── Converters ────────────────────────────────────────────────────────────

func toAtlasNodeItem(n db.ZdxNode) AtlasNodeItem {
	return AtlasNodeItem{
		ID:          n.ID,
		Kind:        n.Kind,
		Slug:        n.Slug,
		Title:       n.Title,
		Description: n.Description,
		CreatedAt:   fmtTS(n.CreatedAt),
		UpdatedAt:   fmtTS(n.UpdatedAt),
	}
}

func toAtlasCoderefItem(r db.ZdxCoderef) AtlasCoderefItem {
	item := AtlasCoderefItem{
		ID:        r.ID,
		File:      r.File,
		Symbol:    r.Symbol,
		Sha:       r.Sha,
		CreatedAt: fmtTS(r.CreatedAt),
	}
	if r.StartLine.Valid {
		v := r.StartLine.Int32
		item.StartLine = &v
	}
	if r.EndLine.Valid {
		v := r.EndLine.Int32
		item.EndLine = &v
	}
	return item
}

func toAtlasChunkItem(c db.ZdxNarrativeChunk) AtlasChunkItem {
	item := AtlasChunkItem{
		ID:           c.ID,
		NodeID:       c.NodeID,
		Title:        c.Title,
		Body:         c.Body,
		ShaAtWrite:   c.ShaAtWrite,
		VerifierKind: c.VerifierKind,
		Status:       c.Status,
		CreatedAt:    fmtTS(c.CreatedAt),
		UpdatedAt:    fmtTS(c.UpdatedAt),
	}
	if c.CoderefID.Valid {
		v := c.CoderefID.Int64
		item.CoderefID = &v
	}
	return item
}

func toAtlasEdgeItem(e db.ZdxEdge) AtlasEdgeItem {
	return AtlasEdgeItem{
		ID:         e.ID,
		FromNodeID: e.FromNodeID,
		ToNodeID:   e.ToNodeID,
		EdgeType:   e.EdgeType,
		Label:      e.Label,
		Weight:     e.Weight,
		CreatedAt:  fmtTS(e.CreatedAt),
	}
}

// ── Routes ────────────────────────────────────────────────────────────────

func (h *Handler) registerAtlasRoutes(api huma.API) {
	// ── Nodes ─────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-atlas-node", Method: http.MethodPost, Path: "/api/projects/{slug}/atlas/nodes"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				Kind        string `json:"kind"`
				Slug        string `json:"slug"`
				Title       string `json:"title,omitempty"`
				Description string `json:"description,omitempty"`
			}
		}) (*struct{ Body AtlasNodeItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.Kind == "" || in.Body.Slug == "" {
				return nil, apiErr(http.StatusBadRequest, "kind and slug are required")
			}
			node, err := h.Q.CreateAtlasNode(ctx, db.CreateAtlasNodeParams{
				ProjectID:   p.ID,
				Kind:        in.Body.Kind,
				Slug:        in.Body.Slug,
				Title:       in.Body.Title,
				Description: in.Body.Description,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body AtlasNodeItem }{Body: toAtlasNodeItem(node)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-atlas-nodes", Method: http.MethodGet, Path: "/api/projects/{slug}/atlas/nodes"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Kind string `query:"kind"`
		}) (*struct {
			Body struct {
				Nodes []AtlasNodeItem `json:"nodes"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListAtlasNodes(ctx, db.ListAtlasNodesParams{
				ProjectID: p.ID,
				Kind:      in.Kind,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AtlasNodeItem, len(rows))
			for i, r := range rows {
				out[i] = toAtlasNodeItem(r)
			}
			return &struct {
				Body struct {
					Nodes []AtlasNodeItem `json:"nodes"`
				}
			}{Body: struct {
				Nodes []AtlasNodeItem `json:"nodes"`
			}{Nodes: out}}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "get-atlas-node", Method: http.MethodGet, Path: "/api/projects/{slug}/atlas/nodes/{kind}/{node_slug}"},
		func(ctx context.Context, in *struct {
			Slug     string `path:"slug"`
			Kind     string `path:"kind"`
			NodeSlug string `path:"node_slug"`
		}) (*struct{ Body AtlasNodeDetail }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			node, err := h.Q.GetAtlasNode(ctx, db.GetAtlasNodeParams{
				ProjectID: p.ID,
				Kind:      in.Kind,
				Slug:      in.NodeSlug,
			})
			if err != nil {
				return nil, apiErr(http.StatusNotFound, "node not found")
			}
			chunks, _ := h.Q.ListAtlasChunks(ctx, node.ID)
			edges, _ := h.Q.ListAtlasEdgesFrom(ctx, node.ID)

			chunkItems := make([]AtlasChunkItem, len(chunks))
			for i, c := range chunks {
				chunkItems[i] = toAtlasChunkItem(c)
			}
			edgeItems := make([]AtlasEdgeFromItem, len(edges))
			for i, e := range edges {
				edgeItems[i] = AtlasEdgeFromItem{
					AtlasEdgeItem: AtlasEdgeItem{
						ID:         e.ID,
						FromNodeID: e.FromNodeID,
						ToNodeID:   e.ToNodeID,
						EdgeType:   e.EdgeType,
						Label:      e.Label,
						Weight:     e.Weight,
						CreatedAt:  fmtTS(e.CreatedAt),
					},
					ToKind: e.ToKind,
					ToSlug: e.ToSlug,
				}
			}
			detail := AtlasNodeDetail{
				AtlasNodeItem: toAtlasNodeItem(node),
				Chunks:        chunkItems,
				Edges:         edgeItems,
			}
			return &struct{ Body AtlasNodeDetail }{Body: detail}, nil
		})

	// ── Stale chunks ──────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "list-stale-atlas-chunks", Method: http.MethodGet, Path: "/api/projects/{slug}/atlas/chunks/stale"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
		}) (*struct {
			Body struct {
				StaleCount int                   `json:"stale_count"`
				Chunks     []AtlasStaleChunkItem `json:"chunks"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListStaleChunksByProject(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AtlasStaleChunkItem, len(rows))
			for i, r := range rows {
				out[i] = AtlasStaleChunkItem{
					ID:       r.ID,
					NodeKind: r.NodeKind,
					NodeSlug: r.NodeSlug,
					Title:    r.Title,
					File:     r.CoderefFile,
				}
			}
			return &struct {
				Body struct {
					StaleCount int                   `json:"stale_count"`
					Chunks     []AtlasStaleChunkItem `json:"chunks"`
				}
			}{Body: struct {
				StaleCount int                   `json:"stale_count"`
				Chunks     []AtlasStaleChunkItem `json:"chunks"`
			}{StaleCount: len(out), Chunks: out}}, nil
		})

	// ── Coderefs ──────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-atlas-coderef", Method: http.MethodPost, Path: "/api/projects/{slug}/atlas/coderefs"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				File      string `json:"file"`
				StartLine *int32 `json:"start_line,omitempty"`
				EndLine   *int32 `json:"end_line,omitempty"`
				Symbol    string `json:"symbol,omitempty"`
				Sha       string `json:"sha,omitempty"`
			}
		}) (*struct{ Body AtlasCoderefItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.File == "" {
				return nil, apiErr(http.StatusBadRequest, "file is required")
			}
			startLine := pgtype.Int4{}
			if in.Body.StartLine != nil {
				startLine = pgtype.Int4{Int32: *in.Body.StartLine, Valid: true}
			}
			endLine := pgtype.Int4{}
			if in.Body.EndLine != nil {
				endLine = pgtype.Int4{Int32: *in.Body.EndLine, Valid: true}
			}
			ref, err := h.Q.CreateAtlasCoderef(ctx, db.CreateAtlasCoderefParams{
				ProjectID: p.ID,
				File:      in.Body.File,
				StartLine: startLine,
				EndLine:   endLine,
				Symbol:    in.Body.Symbol,
				Sha:       in.Body.Sha,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body AtlasCoderefItem }{Body: toAtlasCoderefItem(ref)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-atlas-coderefs", Method: http.MethodGet, Path: "/api/projects/{slug}/atlas/coderefs"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
		}) (*struct {
			Body struct {
				Coderefs []AtlasCoderefItem `json:"coderefs"`
			}
		}, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			rows, err := h.Q.ListAtlasCoderefs(ctx, p.ID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AtlasCoderefItem, len(rows))
			for i, r := range rows {
				out[i] = toAtlasCoderefItem(r)
			}
			return &struct {
				Body struct {
					Coderefs []AtlasCoderefItem `json:"coderefs"`
				}
			}{Body: struct {
				Coderefs []AtlasCoderefItem `json:"coderefs"`
			}{Coderefs: out}}, nil
		})

	// ── Chunks ────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-atlas-chunk", Method: http.MethodPost, Path: "/api/projects/{slug}/atlas/nodes/{node_id}/chunks"},
		func(ctx context.Context, in *struct {
			Slug   string `path:"slug"`
			NodeID int64  `path:"node_id"`
			Body   struct {
				Title        string `json:"title,omitempty"`
				Body         string `json:"body"`
				CoderefID    *int64 `json:"coderef_id,omitempty"`
				ShaAtWrite   string `json:"sha_at_write,omitempty"`
				VerifierKind string `json:"verifier_kind,omitempty"`
			}
		}) (*struct{ Body AtlasChunkItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			verifierKind := in.Body.VerifierKind
			if verifierKind == "" {
				verifierKind = "agent"
			}
			coderefID := pgtype.Int8{}
			if in.Body.CoderefID != nil {
				coderefID = pgtype.Int8{Int64: *in.Body.CoderefID, Valid: true}
			}
			chunk, err := h.Q.CreateAtlasChunk(ctx, db.CreateAtlasChunkParams{
				ProjectID:    p.ID,
				NodeID:       in.NodeID,
				CoderefID:    coderefID,
				Title:        in.Body.Title,
				Body:         in.Body.Body,
				ShaAtWrite:   in.Body.ShaAtWrite,
				VerifierKind: verifierKind,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body AtlasChunkItem }{Body: toAtlasChunkItem(chunk)}, nil
		})

	huma.Register(api, huma.Operation{OperationID: "list-atlas-chunks", Method: http.MethodGet, Path: "/api/projects/{slug}/atlas/nodes/{node_id}/chunks"},
		func(ctx context.Context, in *struct {
			Slug   string `path:"slug"`
			NodeID int64  `path:"node_id"`
		}) (*struct {
			Body struct {
				Chunks []AtlasChunkItem `json:"chunks"`
			}
		}, error) {
			if _, err := getProject(ctx, h.Q, in.Slug); err != nil {
				return nil, err
			}
			rows, err := h.Q.ListAtlasChunks(ctx, in.NodeID)
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			out := make([]AtlasChunkItem, len(rows))
			for i, c := range rows {
				out[i] = toAtlasChunkItem(c)
			}
			return &struct {
				Body struct {
					Chunks []AtlasChunkItem `json:"chunks"`
				}
			}{Body: struct {
				Chunks []AtlasChunkItem `json:"chunks"`
			}{Chunks: out}}, nil
		})

	// ── Edges ─────────────────────────────────────────────────────────────

	huma.Register(api, huma.Operation{OperationID: "create-atlas-edge", Method: http.MethodPost, Path: "/api/projects/{slug}/atlas/edges"},
		func(ctx context.Context, in *struct {
			Slug string `path:"slug"`
			Body struct {
				FromNodeID int64  `json:"from_node_id"`
				ToNodeID   int64  `json:"to_node_id"`
				EdgeType   string `json:"edge_type"`
				Label      string `json:"label,omitempty"`
				Weight     int32  `json:"weight,omitempty"`
			}
		}) (*struct{ Body AtlasEdgeItem }, error) {
			p, err := getProject(ctx, h.Q, in.Slug)
			if err != nil {
				return nil, err
			}
			if in.Body.EdgeType == "" {
				return nil, apiErr(http.StatusBadRequest, "edge_type is required")
			}
			edge, err := h.Q.CreateAtlasEdge(ctx, db.CreateAtlasEdgeParams{
				ProjectID:  p.ID,
				FromNodeID: in.Body.FromNodeID,
				ToNodeID:   in.Body.ToNodeID,
				EdgeType:   in.Body.EdgeType,
				Label:      in.Body.Label,
				Weight:     in.Body.Weight,
			})
			if err != nil {
				return nil, apiErr(500, err.Error())
			}
			return &struct{ Body AtlasEdgeItem }{Body: toAtlasEdgeItem(edge)}, nil
		})
}
