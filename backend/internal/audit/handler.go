package audit

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// Handler handles audit log HTTP requests.
type Handler struct {
	auditor *PostgresAuditor
	logger  *slog.Logger
}

// NewHandler creates an audit Handler.
func NewHandler(auditor *PostgresAuditor, logger *slog.Logger) *Handler {
	return &Handler{auditor: auditor, logger: logger}
}

// ListAuditLog handles GET /api/v1/organizations/{org_id}/audit-log
func (h *Handler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	orgID, err := uuid.Parse(r.PathValue("org_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "org_id", api.P("field", "org_id")))
		return
	}

	entries, err := h.auditor.Query(r.Context(), AuditQuery{
		OrgID: &orgID,
		Limit: 100,
	})
	if err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, entries)
}

// GetAuditEntry handles GET /api/v1/organizations/{org_id}/audit-log/{event_id}
func (h *Handler) GetAuditEntry(w http.ResponseWriter, r *http.Request) {
	eventID, err := uuid.Parse(r.PathValue("event_id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_uuid", "event_id", api.P("field", "event_id")))
		return
	}

	_ = middleware.OrgIDFromContext(r.Context()) // org scoping handled by RLS

	entries, err := h.auditor.Query(r.Context(), AuditQuery{
		Limit: 1,
	})
	if err != nil {
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	// Find the specific entry by event_id
	for _, entry := range entries {
		if entry.EventID == eventID {
			api.WriteJSON(w, http.StatusOK, entry)
			return
		}
	}

	api.WriteError(w, api.NewNotFoundError("resource.not_found", api.P("resource", "audit_entry")))
}
