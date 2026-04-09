package org

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
)

// AmenityHandler handles amenity and reservation HTTP requests.
type AmenityHandler struct {
	service *OrgService
	logger  *slog.Logger
}

// NewAmenityHandler constructs an AmenityHandler backed by the given service.
func NewAmenityHandler(service *OrgService, logger *slog.Logger) *AmenityHandler {
	return &AmenityHandler{service: service, logger: logger}
}

// ─── Amenity CRUD ─────────────────────────────────────────────────────────────

// CreateAmenity handles POST /api/v1/organizations/{org_id}/amenities.
func (h *AmenityHandler) CreateAmenity(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateAmenityRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateAmenity(r.Context(), orgID, req)
	if err != nil {
		h.logger.Error("CreateAmenity failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListAmenities handles GET /api/v1/organizations/{org_id}/amenities.
func (h *AmenityHandler) ListAmenities(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	page := api.ParsePageRequest(r)

	afterID, err := parseCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	amenities, hasMore, err := h.service.ListAmenities(r.Context(), orgID, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListAmenities failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore && len(amenities) > 0 {
		meta = &api.Meta{
			Cursor:  api.EncodeCursor(map[string]string{"id": amenities[len(amenities)-1].ID.String()}),
			HasMore: true,
		}
	}

	api.WriteJSONWithMeta(w, http.StatusOK, amenities, meta)
}

// GetAmenity handles GET /api/v1/organizations/{org_id}/amenities/{amenity_id}.
func (h *AmenityHandler) GetAmenity(w http.ResponseWriter, r *http.Request) {
	amenityID, err := parseAmenityID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	amenity, err := h.service.GetAmenity(r.Context(), amenityID)
	if err != nil {
		h.logger.Error("GetAmenity failed", "amenity_id", amenityID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, amenity)
}

// UpdateAmenity handles PATCH /api/v1/organizations/{org_id}/amenities/{amenity_id}.
func (h *AmenityHandler) UpdateAmenity(w http.ResponseWriter, r *http.Request) {
	amenityID, err := parseAmenityID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateAmenityRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateAmenity(r.Context(), amenityID, req)
	if err != nil {
		h.logger.Error("UpdateAmenity failed", "amenity_id", amenityID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// DeleteAmenity handles DELETE /api/v1/organizations/{org_id}/amenities/{amenity_id}.
func (h *AmenityHandler) DeleteAmenity(w http.ResponseWriter, r *http.Request) {
	amenityID, err := parseAmenityID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	if err := h.service.DeleteAmenity(r.Context(), amenityID); err != nil {
		h.logger.Error("DeleteAmenity failed", "amenity_id", amenityID, "error", err)
		api.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ─── Reservations ─────────────────────────────────────────────────────────────

// CreateReservation handles POST /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations.
func (h *AmenityHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	amenityID, err := parseAmenityID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req CreateReservationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateReservation(r.Context(), orgID, amenityID, req)
	if err != nil {
		h.logger.Error("CreateReservation failed", "amenity_id", amenityID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListAmenityReservations handles GET /api/v1/organizations/{org_id}/amenities/{amenity_id}/reservations.
func (h *AmenityHandler) ListAmenityReservations(w http.ResponseWriter, r *http.Request) {
	amenityID, err := parseAmenityID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	reservations, err := h.service.ListAmenityReservations(r.Context(), amenityID)
	if err != nil {
		h.logger.Error("ListAmenityReservations failed", "amenity_id", amenityID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, reservations)
}

// ListOrgReservations handles GET /api/v1/organizations/{org_id}/reservations.
// Returns all reservations for the authenticated user within the org.
func (h *AmenityHandler) ListOrgReservations(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseOrgID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())

	reservations, err := h.service.ListUserReservations(r.Context(), orgID, userID)
	if err != nil {
		h.logger.Error("ListOrgReservations failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, reservations)
}

// GetReservation handles GET /api/v1/organizations/{org_id}/reservations/{reservation_id}.
func (h *AmenityHandler) GetReservation(w http.ResponseWriter, r *http.Request) {
	reservationID, err := parseReservationID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	reservation, err := h.service.GetReservation(r.Context(), reservationID)
	if err != nil {
		h.logger.Error("GetReservation failed", "reservation_id", reservationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, reservation)
}

// UpdateReservation handles PATCH /api/v1/organizations/{org_id}/reservations/{reservation_id}.
func (h *AmenityHandler) UpdateReservation(w http.ResponseWriter, r *http.Request) {
	reservationID, err := parseReservationID(r)
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var req UpdateReservationRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateReservation(r.Context(), reservationID, req)
	if err != nil {
		h.logger.Error("UpdateReservation failed", "reservation_id", reservationID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// ─── Path value helpers ───────────────────────────────────────────────────────

// parseAmenityID extracts and parses the {amenity_id} path value from the request.
func parseAmenityID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("amenity_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("amenity_id is required", "amenity_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("amenity_id must be a valid UUID", "amenity_id")
	}
	return id, nil
}

// parseReservationID extracts and parses the {reservation_id} path value from the request.
func parseReservationID(r *http.Request) (uuid.UUID, error) {
	raw := r.PathValue("reservation_id")
	if raw == "" {
		return uuid.Nil, api.NewValidationError("reservation_id is required", "reservation_id")
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("reservation_id must be a valid UUID", "reservation_id")
	}
	return id, nil
}
