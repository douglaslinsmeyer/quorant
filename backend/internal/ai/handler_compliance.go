package ai

import (
	"log/slog"
	"net/http"

	"github.com/quorant/quorant/internal/platform/api"
)

// ComplianceHandler handles tenant-scoped HTTP requests for compliance endpoints.
type ComplianceHandler struct {
	service *ComplianceService
	checks  ComplianceCheckRepository
	logger  *slog.Logger
}

// NewComplianceHandler constructs a ComplianceHandler.
func NewComplianceHandler(service *ComplianceService, checks ComplianceCheckRepository, logger *slog.Logger) *ComplianceHandler {
	return &ComplianceHandler{service: service, checks: checks, logger: logger}
}

// GetComplianceReport handles GET /api/v1/organizations/{org_id}/compliance.
// Returns a full compliance report across all 7 categories for the org.
func (h *ComplianceHandler) GetComplianceReport(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	report, err := h.service.EvaluateCompliance(r.Context(), orgID)
	if err != nil {
		h.logger.Error("GetComplianceReport failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, report)
}

// CheckCategory handles GET /api/v1/organizations/{org_id}/compliance/{category}.
// Evaluates compliance for a single category.
func (h *ComplianceHandler) CheckCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	category := r.PathValue("category")
	if !IsValidRuleCategory(category) {
		api.WriteError(w, api.NewValidationError("validation.invalid", "category", api.P("field", "category")))
		return
	}

	result, err := h.service.CheckCompliance(r.Context(), orgID, category)
	if err != nil {
		h.logger.Error("CheckCategory failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, result)
}

// GetComplianceHistory handles GET /api/v1/organizations/{org_id}/compliance/history.
// Returns paginated compliance check records for the org.
func (h *ComplianceHandler) GetComplianceHistory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	page := api.ParsePageRequest(r)

	afterID, err := parseComplianceCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_cursor", "cursor"))
		return
	}

	checks, hasMore, err := h.checks.ListByOrg(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("GetComplianceHistory failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	var nextCursor string
	if hasMore && len(checks) > 0 {
		last := checks[len(checks)-1]
		nextCursor = api.EncodeCursor(map[string]string{"id": last.ID.String()})
	}

	api.WriteJSONWithMeta(w, http.StatusOK, checks, &api.Meta{
		Cursor:  nextCursor,
		HasMore: hasMore,
	})
}

// ListJurisdictionRulesForOrg handles GET /api/v1/organizations/{org_id}/jurisdiction-rules.
// Returns all active rules for the org's jurisdiction.
func (h *ComplianceHandler) ListJurisdictionRulesForOrg(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	o, err := h.service.orgLookup.FindByID(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesForOrg FindByID failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if o == nil {
		api.WriteError(w, api.NewNotFoundError("resource.not_found", api.P("resource", "organization")))
		return
	}
	if o.State == nil {
		api.WriteError(w, api.NewValidationError("validation.required", "state", api.P("field", "state")))
		return
	}

	rules, err := h.service.rules.ListActiveRulesByJurisdiction(r.Context(), *o.State)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesForOrg ListActiveRulesByJurisdiction failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, rules)
}

// ListJurisdictionRulesByCategory handles GET /api/v1/organizations/{org_id}/jurisdiction-rules/{category}.
// Returns active rules for the org's jurisdiction filtered to a single category.
func (h *ComplianceHandler) ListJurisdictionRulesByCategory(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	category := r.PathValue("category")
	if !IsValidRuleCategory(category) {
		api.WriteError(w, api.NewValidationError("validation.invalid", "category", api.P("field", "category")))
		return
	}

	o, err := h.service.orgLookup.FindByID(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesByCategory FindByID failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if o == nil {
		api.WriteError(w, api.NewNotFoundError("resource.not_found", api.P("resource", "organization")))
		return
	}
	if o.State == nil {
		api.WriteError(w, api.NewValidationError("validation.required", "state", api.P("field", "state")))
		return
	}

	rules, err := h.service.rules.ListActiveRules(r.Context(), *o.State, category)
	if err != nil {
		h.logger.Error("ListJurisdictionRulesByCategory ListActiveRules failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, rules)
}
