package ai

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// AIHandler handles HTTP requests for all AI endpoints.
type AIHandler struct {
	policyService  *PolicyService
	contextService *ContextLakeService
	policyResolver *PostgresPolicyResolver
	orgRepo OrgLookup
	logger         *slog.Logger
}

// NewAIHandler constructs an AIHandler.
func NewAIHandler(policyService *PolicyService, contextService *ContextLakeService, orgRepo OrgLookup, logger *slog.Logger) *AIHandler {
	return &AIHandler{
		policyService:  policyService,
		contextService: contextService,
		policyResolver: NewPostgresPolicyResolver(policyService),
		orgRepo:        orgRepo,
		logger:         logger,
	}
}

// parseOrgID extracts org_id from the URL path values.
func parseOrgID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(r.PathValue("org_id"))
}

// ─── Governing Documents ──────────────────────────────────────────────────────

// RegisterGoverningDoc handles POST /organizations/{org_id}/governing-documents
func (h *AIHandler) RegisterGoverningDoc(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	var req RegisterGoverningDocRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	doc, err := h.policyService.RegisterGoverningDoc(r.Context(), orgID, req)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, doc)
}

// ListGoverningDocs handles GET /organizations/{org_id}/governing-documents
func (h *AIHandler) ListGoverningDocs(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	docs, err := h.policyService.ListGoverningDocs(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, docs)
}

// GetGoverningDoc handles GET /organizations/{org_id}/governing-documents/{doc_id}
func (h *AIHandler) GetGoverningDoc(w http.ResponseWriter, r *http.Request) {
	docID, err := uuid.Parse(r.PathValue("doc_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("doc_id must be a valid UUID", "doc_id"))
		return
	}

	doc, err := h.policyService.GetGoverningDoc(r.Context(), docID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, doc)
}

// RemoveGoverningDoc handles DELETE /organizations/{org_id}/governing-documents/{doc_id}
// Per the arch doc: "supersede, not delete" — marks the document as superseded
// by updating its indexing_status and leaving it in place for audit history.
func (h *AIHandler) RemoveGoverningDoc(w http.ResponseWriter, r *http.Request) {
	docID, err := uuid.Parse(r.PathValue("doc_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("doc_id must be a valid UUID", "doc_id"))
		return
	}

	doc, err := h.policyService.GetGoverningDoc(r.Context(), docID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	// Mark as superseded by setting indexing_status to a terminal state
	indexedAt := time.Now()
	doc.IndexingStatus = "failed" // reuse "failed" status to indicate no longer active
	doc.IndexedAt = &indexedAt
	if _, err := h.policyService.UpdateGoverningDoc(r.Context(), doc); err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ReindexGoverningDoc handles POST /organizations/{org_id}/governing-documents/{doc_id}/reindex
func (h *AIHandler) ReindexGoverningDoc(w http.ResponseWriter, r *http.Request) {
	docID, err := uuid.Parse(r.PathValue("doc_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("doc_id must be a valid UUID", "doc_id"))
		return
	}

	doc, err := h.policyService.ReindexGoverningDoc(r.Context(), docID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, doc)
}

// ─── Policy Extractions ───────────────────────────────────────────────────────

// ListExtractions handles GET /organizations/{org_id}/policy-extractions
func (h *AIHandler) ListExtractions(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	extractions, err := h.policyService.ListExtractions(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extractions)
}

// GetExtraction handles GET /organizations/{org_id}/policy-extractions/{extraction_id}
func (h *AIHandler) GetExtraction(w http.ResponseWriter, r *http.Request) {
	extractionID, err := uuid.Parse(r.PathValue("extraction_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("extraction_id must be a valid UUID", "extraction_id"))
		return
	}

	extraction, err := h.policyService.GetExtraction(r.Context(), extractionID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extraction)
}

// ApproveExtraction handles POST /organizations/{org_id}/policy-extractions/{extraction_id}/approve
func (h *AIHandler) ApproveExtraction(w http.ResponseWriter, r *http.Request) {
	extractionID, err := uuid.Parse(r.PathValue("extraction_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("extraction_id must be a valid UUID", "extraction_id"))
		return
	}

	extraction, err := h.policyService.ApproveExtraction(r.Context(), extractionID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extraction)
}

// RejectExtraction handles POST /organizations/{org_id}/policy-extractions/{extraction_id}/reject
func (h *AIHandler) RejectExtraction(w http.ResponseWriter, r *http.Request) {
	extractionID, err := uuid.Parse(r.PathValue("extraction_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("extraction_id must be a valid UUID", "extraction_id"))
		return
	}

	extraction, err := h.policyService.RejectExtraction(r.Context(), extractionID, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extraction)
}

// ModifyExtraction handles POST /organizations/{org_id}/policy-extractions/{extraction_id}/modify
func (h *AIHandler) ModifyExtraction(w http.ResponseWriter, r *http.Request) {
	extractionID, err := uuid.Parse(r.PathValue("extraction_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("extraction_id must be a valid UUID", "extraction_id"))
		return
	}

	var req ModifyExtractionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	extraction, err := h.policyService.ModifyExtraction(r.Context(), extractionID, req.Override, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extraction)
}

// ─── Active Policies ──────────────────────────────────────────────────────────

// GetActivePolicy handles GET /organizations/{org_id}/policies/{policy_key}
func (h *AIHandler) GetActivePolicy(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	policyKey := r.PathValue("policy_key")
	if policyKey == "" {
		api.WriteError(w, api.NewValidationError("policy_key is required", "policy_key"))
		return
	}

	result, err := h.policyService.GetActivePolicy(r.Context(), orgID, policyKey)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	if result == nil {
		api.WriteError(w, api.NewNotFoundError("no active policy found for key: "+policyKey))
		return
	}

	api.WriteJSON(w, http.StatusOK, result)
}

// ListActivePolicies handles GET /organizations/{org_id}/policies
func (h *AIHandler) ListActivePolicies(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	policies, err := h.policyService.ListActivePolicies(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, policies)
}

// QueryPolicy handles POST /organizations/{org_id}/policy-query
func (h *AIHandler) QueryPolicy(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	var body struct {
		Query   string       `json:"query"`
		Context QueryContext `json:"context"`
	}
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}
	if body.Query == "" {
		api.WriteError(w, api.NewValidationError("query is required", "query"))
		return
	}

	result, err := h.policyResolver.QueryPolicy(r.Context(), orgID, body.Query, body.Context)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	// Placeholder: QueryPolicy returns nil until LLM inference is wired.
	api.WriteJSON(w, http.StatusOK, result)
}

// ─── Policy Resolutions ───────────────────────────────────────────────────────

// ListResolutions handles GET /organizations/{org_id}/policy-resolutions
func (h *AIHandler) ListResolutions(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	resolutions, err := h.policyService.ListResolutions(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, resolutions)
}

// GetResolution handles GET /organizations/{org_id}/policy-resolutions/{resolution_id}
func (h *AIHandler) GetResolution(w http.ResponseWriter, r *http.Request) {
	resolutionID, err := uuid.Parse(r.PathValue("resolution_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("resolution_id must be a valid UUID", "resolution_id"))
		return
	}

	resolution, err := h.policyService.GetResolution(r.Context(), resolutionID)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, resolution)
}

// DecideResolution handles POST /organizations/{org_id}/policy-resolutions/{resolution_id}/decide
func (h *AIHandler) DecideResolution(w http.ResponseWriter, r *http.Request) {
	resolutionID, err := uuid.Parse(r.PathValue("resolution_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("resolution_id must be a valid UUID", "resolution_id"))
		return
	}

	var req DecideResolutionRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	resolution, err := h.policyService.DecideResolution(r.Context(), resolutionID, req.Decision, middleware.UserIDFromContext(r.Context()))
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, resolution)
}

// ─── AI Config ────────────────────────────────────────────────────────────────

// GetAIConfig handles GET /organizations/{org_id}/ai/config
func (h *AIHandler) GetAIConfig(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	o, err := h.orgRepo.FindByID(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	if o == nil {
		api.WriteError(w, api.NewNotFoundError("organization not found"))
		return
	}

	cfg := extractAIConfig(o.Settings)
	api.WriteJSON(w, http.StatusOK, cfg)
}

// UpdateAIConfig handles PUT /organizations/{org_id}/ai/config
func (h *AIHandler) UpdateAIConfig(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("org_id must be a valid UUID", "org_id"))
		return
	}

	var cfg AIConfig
	if err := api.ReadJSON(r, &cfg); err != nil {
		api.WriteError(w, err)
		return
	}

	o, err := h.orgRepo.FindByID(r.Context(), orgID)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	if o == nil {
		api.WriteError(w, api.NewNotFoundError("organization not found"))
		return
	}

	if o.Settings == nil {
		o.Settings = make(map[string]any)
	}

	// Marshal AIConfig to a map[string]any so it can be stored in JSONB settings.
	raw, err := json.Marshal(cfg)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	var cfgMap map[string]any
	if err := json.Unmarshal(raw, &cfgMap); err != nil {
		api.WriteError(w, err)
		return
	}
	o.Settings["ai_config"] = cfgMap

	updated, err := h.orgRepo.Update(r.Context(), o)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, extractAIConfig(updated.Settings))
}

// extractAIConfig reads the "ai_config" key from org settings, returning defaults
// if the key is absent or malformed.
func extractAIConfig(settings map[string]any) AIConfig {
	if settings == nil {
		return DefaultAIConfig()
	}

	raw, ok := settings["ai_config"]
	if !ok {
		return DefaultAIConfig()
	}

	// Re-marshal the raw value so we can unmarshal into typed struct.
	b, err := json.Marshal(raw)
	if err != nil {
		return DefaultAIConfig()
	}

	var cfg AIConfig
	if err := json.Unmarshal(b, &cfg); err != nil {
		return DefaultAIConfig()
	}
	return cfg
}
