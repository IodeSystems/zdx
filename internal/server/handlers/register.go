package handlers

import "github.com/danielgtaylor/huma/v2"

// Register wires every API route group in this package onto the given API.
// Callers (server.registerRoutes) pass their concrete Deps, which satisfies the
// Broker / Reconciler / Embedder / IngestRegistrar interfaces. The constructed
// Handler is returned so callers can attach non-huma routes (e.g. the agent
// WebSocket endpoint) to the same instance.
func Register(api huma.API, deps *Deps) *Handler {
	h := &Handler{Deps: deps}
	h.registerAuthRoutes(api)
	h.registerIssueRoutes(api)
	h.registerTaskRoutes(api)
	h.registerFeatureRoutes(api)
	h.registerProjectRoutes(api)
	h.registerFocusRoutes(api)
	h.registerDxRoutes(api)
	h.registerErrorRoutes(api)
	h.registerCommentRoutes(api)
	h.registerEventRoutes(api)
	h.registerAdminRoutes(api)
	h.registerCodeRefRoutes(api)
	h.registerQARoutes(api)
	h.registerClaudeRoutes(api)
	h.registerAgentSessionRoutes(api)
	h.registerCounterRoutes(api)
	h.registerKpiRoutes(api)
	h.registerErrorEventRoutes(api)
	h.registerLogEventRoutes(api)
	h.registerAgentRoutes(api)
	h.registerPatternRoutes(api)
	h.registerConcernRoutes(api)
	h.registerSoloRoutes(api)
	h.registerTodoRoutes(api)
	h.registerHistoryRoutes(api)
	h.registerPlanRoutes(api)
	h.registerProposalRoutes(api)
	h.registerDiscussionRoutes(api)
	h.registerDoctorRoutes(api)
	h.registerMaturityRoutes(api)
	h.registerMaturityQuestionRoutes(api)
	h.registerEnvironmentRoutes(api)
	h.registerFileRoutes()
	h.registerGitProxyRoutes()
	return h
}
