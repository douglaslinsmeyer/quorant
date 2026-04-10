package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// VendorHandler handles vendor and vendor assignment HTTP requests.
type VendorHandler struct {
	service Service
	logger  *slog.Logger
}

// NewVendorHandler constructs a VendorHandler backed by the given service.
func NewVendorHandler(service Service, logger *slog.Logger) *VendorHandler {
	return &VendorHandler{service: service, logger: logger}
}

// ─── Vendor CRUD ──────────────────────────────────────────────────────────────

// CreateVendor handles POST /api/v1/vendors.
func (h *VendorHandler) CreateVendor(w http.ResponseWriter, r *http.Request) {
	var req CreateVendorRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateVendor(r.Context(), req)
	if err != nil {
		h.logger.Error("CreateVendor failed", "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListVendors handles GET /api/v1/vendors.
func (h *VendorHandler) ListVendors(w http.ResponseWriter, r *http.Request) {
	page := api.ParsePageRequest(r)

	afterID, err := parseCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("validation.invalid_cursor", "cursor"))
		return
	}

	vendors, hasMore, err := h.service.ListVendors(r.Context(), page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListVendors failed", "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(vendors) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": vendors[len(vendors)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, vendors, meta)
}

// GetVendor handles GET /api/v1/vendors/{vendor_id}.
func (h *VendorHandler) GetVendor(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parseVendorID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	vendor, err := h.service.GetVendor(r.Context(), vendorID)
	if err != nil {
		h.logger.Error("GetVendor failed", "vendor_id", vendorID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, vendor)
}

// UpdateVendor handles PATCH /api/v1/vendors/{vendor_id}.
func (h *VendorHandler) UpdateVendor(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parseVendorID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateVendorRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateVendor(r.Context(), vendorID, req)
	if err != nil {
		h.logger.Error("UpdateVendor failed", "vendor_id", vendorID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteVendor handles DELETE /api/v1/vendors/{vendor_id}.
func (h *VendorHandler) DeleteVendor(w http.ResponseWriter, r *http.Request) {
	vendorID, err := parseVendorID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteVendor(r.Context(), vendorID); err != nil {
		h.logger.Error("DeleteVendor failed", "vendor_id", vendorID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Vendor Assignments ───────────────────────────────────────────────────────

// CreateVendorAssignment handles POST /api/v1/organizations/{org_id}/vendor-assignments.
func (h *VendorHandler) CreateVendorAssignment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateVendorAssignmentRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateVendorAssignment(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateVendorAssignment failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListVendorAssignments handles GET /api/v1/organizations/{org_id}/vendor-assignments.
func (h *VendorHandler) ListVendorAssignments(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	assignments, err := h.service.ListVendorAssignments(r.Context(), orgID)
	if err != nil {
		h.logger.Error("ListVendorAssignments failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, assignments)
}

// DeleteVendorAssignment handles DELETE /api/v1/organizations/{org_id}/vendor-assignments/{id}.
func (h *VendorHandler) DeleteVendorAssignment(w http.ResponseWriter, r *http.Request) {
	assignmentID, err := parseAssignmentID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteVendorAssignment(r.Context(), assignmentID); err != nil {
		h.logger.Error("DeleteVendorAssignment failed", "assignment_id", assignmentID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Path value helpers ───────────────────────────────────────────────────────

// parseVendorID extracts and parses the {vendor_id} path value from the request.
func parseVendorID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("vendor_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", "vendor_id", api.P("field", "vendor_id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", "vendor_id", api.P("field", "vendor_id"))
	}
	return id, nil
}

// parseAssignmentID extracts and parses the {id} path value for assignments.
func parseAssignmentID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("validation.required", "id", api.P("field", "id"))
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("validation.invalid_uuid", "id", api.P("field", "id"))
	}
	return id, nil
}
