package ai

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/auth"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// RegisterRoutes registers all AI module routes on the mux.
// All routes require authentication and tenant context middleware.
// All AI endpoints are gated by the ai.context_lake entitlement.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *AIHandler,
	validator auth.TokenValidator,
	checker middleware.PermissionChecker,
	resolveUserID func(context.Context) (uuid.UUID, error),
	entChecker middleware.EntitlementChecker,
) {
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequireEntitlement(entChecker, "ai.context_lake")(
					middleware.RequirePermission(checker, perm, resolveUserID)(
						http.HandlerFunc(h)))))
	}

	// Governing documents
	mux.Handle("POST /api/v1/organizations/{org_id}/governing-documents", permMw("ai.governing_doc.manage", handler.RegisterGoverningDoc))
	mux.Handle("GET /api/v1/organizations/{org_id}/governing-documents", permMw("ai.governing_doc.read", handler.ListGoverningDocs))
	mux.Handle("GET /api/v1/organizations/{org_id}/governing-documents/{doc_id}", permMw("ai.governing_doc.read", handler.GetGoverningDoc))
	mux.Handle("DELETE /api/v1/organizations/{org_id}/governing-documents/{doc_id}", permMw("ai.governing_doc.manage", handler.RemoveGoverningDoc))
	mux.Handle("POST /api/v1/organizations/{org_id}/governing-documents/{doc_id}/reindex", permMw("ai.governing_doc.manage", handler.ReindexGoverningDoc))

	// Policy extractions
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-extractions", permMw("ai.extraction.read", handler.ListExtractions))
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}", permMw("ai.extraction.read", handler.GetExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/approve", permMw("ai.extraction.review", handler.ApproveExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/reject", permMw("ai.extraction.review", handler.RejectExtraction))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-extractions/{extraction_id}/modify", permMw("ai.extraction.review", handler.ModifyExtraction))

	// Active policies
	mux.Handle("GET /api/v1/organizations/{org_id}/policies/{policy_key}", permMw("ai.policy.read", handler.GetActivePolicy))
	mux.Handle("GET /api/v1/organizations/{org_id}/policies", permMw("ai.policy.read", handler.ListActivePolicies))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-query", permMw("ai.policy.query", handler.QueryPolicy))

	// Policy resolutions
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-resolutions", permMw("ai.policy.read", handler.ListResolutions))
	mux.Handle("GET /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}", permMw("ai.policy.read", handler.GetResolution))
	mux.Handle("POST /api/v1/organizations/{org_id}/policy-resolutions/{resolution_id}/decide", permMw("ai.extraction.review", handler.DecideResolution))

	// AI config
	mux.Handle("GET /api/v1/organizations/{org_id}/ai/config", permMw("ai.config.manage", handler.GetAIConfig))
	mux.Handle("PUT /api/v1/organizations/{org_id}/ai/config", permMw("ai.config.manage", handler.UpdateAIConfig))
}
