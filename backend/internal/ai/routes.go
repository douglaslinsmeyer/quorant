package ai

import (
	"net/http"

	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all AI module routes on the mux.
// All routes require authentication and tenant context middleware.
func RegisterRoutes(mux *http.ServeMux, handler *AIHandler, validator auth.TokenValidator) {
	orgMw := func(h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator, middleware.TenantContext(http.HandlerFunc(h)))
	}

	// Governing documents
	mux.Handle("POST /api/v1/organizations/{org_id}/governing-documents", orgMw(handler.RegisterGoverningDoc))
	mux.Handle("GET /api/v1/organizations/{org_id}/governing-documents", orgMw(handler.ListGoverningDocs))
	mux.Handle("GET /api/v1/organizations/{org_id}/governing-documents/{doc_id}", orgMw(handler.GetGoverningDoc))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/governing-documents/{doc_id}", orgMw(handler.RemoveGoverningDoc))
	mux.Handle("POST /api/v1/organizations/{org_id}/governing-documents/{doc_id}/reindex", orgMw(handler.ReindexGoverningDoc))

	// Policy extractions
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-extractions", orgMw(handler.ListExtractions))
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}", orgMw(handler.GetExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/approve", orgMw(handler.ApproveExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/reject", orgMw(handler.RejectExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/modify", orgMw(handler.ModifyExtraction))

	// Active policies
	mux.Handle("GET /api/v1/organizations/{org_id}/policies/{policy_key}", orgMw(handler.GetActivePolicy))
	mux.Handle("GET /api/v1/organizations/{org_id}/policies", orgMw(handler.ListActivePolicies))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-query", orgMw(handler.QueryPolicy))

	// Policy resolutions
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-resolutions", orgMw(handler.ListResolutions))
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}", orgMw(handler.GetResolution))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}/decide", orgMw(handler.DecideResolution))

	// AI config
	mux.Handle("GET /api/v1/organizations/{org_id}/ai/config", orgMw(handler.GetAIConfig))
	mux.Handle("PUT /api/v1/organizations/{org_id}/ai/config", orgMw(handler.UpdateAIConfig))
}
