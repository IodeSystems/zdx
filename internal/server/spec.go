package server

import (
	"encoding/json"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/go-chi/chi/v5"

	"github.com/iodesystems/zdx-go/internal/server/handlers"
)

// SpecJSON returns the server's OpenAPI spec as JSON without requiring a
// database connection. All route groups are registered (including ingest and
// WS routes) so the output matches a full server start. Handler closures
// capture the server struct but are never invoked.
func SpecJSON() ([]byte, error) {
	mux := chi.NewMux()
	cfg := huma.DefaultConfig("ZDX API", "1.0.0")
	cfg.Info.Description = "zdx developer-experience platform API"
	api := humachi.New(mux, cfg)

	s := &Server{mux: mux}
	handlers.Register(api, &handlers.Deps{Mux: mux})
	s.registerIngestRoutes(api)
	s.registerCounterIngestRoutes(api)
	s.registerErrorIngestRoutes(api)
	s.registerLogIngestRoutes(api)
	s.registerPromIngestRoutes(api)
	s.registerWSRoutes(api)

	return json.Marshal(api.OpenAPI())
}
