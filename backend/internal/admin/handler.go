package admin

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// AdminHandler handles admin HTTP requests.
type AdminHandler struct {
	service *AdminService
	logger  *slog.Logger
}

// NewAdminHandler constructs an AdminHandler backed by the given service and logger.
func NewAdminHandler(service *AdminService, logger *slog.Logger) *AdminHandler {
	return &AdminHandler{service: service, logger: logger}
}

// ─── Tenant handlers ──────────────────────────────────────────────────────────

// ListTenants handles GET /api/v1/admin/tenants.
func (h *AdminHandler) ListTenants(w http.ResponseWriter, r *http.Request) {
	tenants, err := h.service.ListTenants(r.Context())
	if err != nil {
		h.logger.Error("listing tenants", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, tenants)
}

// GetTenantDashboard handles GET /api/v1/admin/tenants/{org_id}.
func (h *AdminHandler) GetTenantDashboard(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("org_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid org_id", "org_id"))
		return
	}

	dashboard, err := h.service.GetTenantDashboard(r.Context(), orgID)
	if err != nil {
		h.logger.Error("getting tenant dashboard", "error", err, "org_id", orgID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, dashboard)
}

// SuspendTenant handles POST /api/v1/admin/tenants/{org_id}/suspend.
func (h *AdminHandler) SuspendTenant(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("org_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid org_id", "org_id"))
		return
	}

	result, err := h.service.SuspendTenant(r.Context(), orgID)
	if err != nil {
		h.logger.Error("suspending tenant", "error", err, "org_id", orgID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// ReactivateTenant handles POST /api/v1/admin/tenants/{org_id}/reactivate.
func (h *AdminHandler) ReactivateTenant(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("org_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid org_id", "org_id"))
		return
	}

	result, err := h.service.ReactivateTenant(r.Context(), orgID)
	if err != nil {
		h.logger.Error("reactivating tenant", "error", err, "org_id", orgID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// ─── Impersonation handlers ───────────────────────────────────────────────────

// StartImpersonation handles POST /api/v1/admin/impersonate.
func (h *AdminHandler) StartImpersonation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		TargetUserID string `json:"target_user_id"`
	}
	if err := api.ReadJSON(r, &body); err != nil {
		api.WriteError(w, err)
		return
	}
	if body.TargetUserID == "" {
		api.WriteError(w, api.NewValidationError("target_user_id is required", "target_user_id"))
		return
	}
	targetUserID, err := uuid.Parse(body.TargetUserID)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid target_user_id", "target_user_id"))
		return
	}

	result, err := h.service.StartImpersonation(r.Context(), targetUserID)
	if err != nil {
		h.logger.Error("starting impersonation", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// StopImpersonation handles POST /api/v1/admin/impersonate/stop.
func (h *AdminHandler) StopImpersonation(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.StopImpersonation(r.Context())
	if err != nil {
		h.logger.Error("stopping impersonation", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// ─── User admin handlers ──────────────────────────────────────────────────────

// SearchUsers handles GET /api/v1/admin/users.
func (h *AdminHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	result, err := h.service.SearchUsers(r.Context(), query)
	if err != nil {
		h.logger.Error("searching users", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// ResetPassword handles POST /api/v1/admin/users/{user_id}/reset-password.
func (h *AdminHandler) ResetPassword(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid user_id", "user_id"))
		return
	}

	result, err := h.service.ResetPassword(r.Context(), userID)
	if err != nil {
		h.logger.Error("resetting password", "error", err, "user_id", userID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// UnlockAccount handles POST /api/v1/admin/users/{user_id}/unlock.
func (h *AdminHandler) UnlockAccount(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(r.PathValue("user_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid user_id", "user_id"))
		return
	}

	result, err := h.service.UnlockAccount(r.Context(), userID)
	if err != nil {
		h.logger.Error("unlocking account", "error", err, "user_id", userID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, result)
}

// ─── Feature flag handlers ────────────────────────────────────────────────────

// ListFlags handles GET /api/v1/admin/feature-flags.
func (h *AdminHandler) ListFlags(w http.ResponseWriter, r *http.Request) {
	flags, err := h.service.ListFlags(r.Context())
	if err != nil {
		h.logger.Error("listing feature flags", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusOK, flags)
}

// CreateFlag handles POST /api/v1/admin/feature-flags.
func (h *AdminHandler) CreateFlag(w http.ResponseWriter, r *http.Request) {
	var req CreateFeatureFlagRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), "key"))
		return
	}

	flag, err := h.service.CreateFlag(r.Context(), req)
	if err != nil {
		h.logger.Error("creating feature flag", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusCreated, flag)
}

// UpdateFlag handles PATCH /api/v1/admin/feature-flags/{flag_id}.
func (h *AdminHandler) UpdateFlag(w http.ResponseWriter, r *http.Request) {
	flagID, err := uuid.Parse(r.PathValue("flag_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid flag_id", "flag_id"))
		return
	}

	var req UpdateFeatureFlagRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	flag, err := h.service.UpdateFlag(r.Context(), flagID, req)
	if err != nil {
		h.logger.Error("updating feature flag", "error", err, "flag_id", flagID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if flag == nil {
		api.WriteError(w, api.NewNotFoundError("feature flag not found"))
		return
	}
	api.WriteJSON(w, http.StatusOK, flag)
}

// SetOverride handles POST /api/v1/admin/feature-flags/{flag_id}/overrides.
func (h *AdminHandler) SetOverride(w http.ResponseWriter, r *http.Request) {
	flagID, err := uuid.Parse(r.PathValue("flag_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid flag_id", "flag_id"))
		return
	}

	var req SetFlagOverrideRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}
	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), "org_id"))
		return
	}

	override, err := h.service.SetOverride(r.Context(), flagID, req)
	if err != nil {
		h.logger.Error("setting flag override", "error", err, "flag_id", flagID)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	api.WriteJSON(w, http.StatusCreated, override)
}
