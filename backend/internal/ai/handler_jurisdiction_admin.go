package ai

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/middleware"
	"github.com/quorant/quorant/internal/platform/queue"
)

// parseComplianceCursorID decodes an opaque cursor token and extracts the "id" field as a UUID.
// Returns nil, nil if the cursor is empty (first page).
func parseComplianceCursorID(cursor string) (*uuid.UUID, error) {
	if cursor == "" {
		return nil, nil
	}
	vals, err := api.DecodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(vals["id"])
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// JurisdictionAdminHandler handles platform-admin CRUD for jurisdiction rules.
type JurisdictionAdminHandler struct {
	rules     JurisdictionRuleRepository
	publisher queue.Publisher
	logger    *slog.Logger
}

// NewJurisdictionAdminHandler constructs a JurisdictionAdminHandler.
func NewJurisdictionAdminHandler(rules JurisdictionRuleRepository, publisher queue.Publisher, logger *slog.Logger) *JurisdictionAdminHandler {
	return &JurisdictionAdminHandler{rules: rules, publisher: publisher, logger: logger}
}

// CreateRule handles POST /api/v1/admin/jurisdiction-rules.
func (h *JurisdictionAdminHandler) CreateRule(w http.ResponseWriter, r *http.Request) {
	var req CreateJurisdictionRuleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		api.WriteError(w, api.NewValidationError("effective_date must be YYYY-MM-DD", "effective_date"))
		return
	}

	rule := &JurisdictionRule{
		Jurisdiction:     req.Jurisdiction,
		RuleCategory:     req.RuleCategory,
		RuleKey:          req.RuleKey,
		ValueType:        req.ValueType,
		Value:            req.Value,
		StatuteReference: req.StatuteReference,
		EffectiveDate:    effectiveDate,
		Notes:            req.Notes,
	}

	if req.SourceDocID != "" {
		sourceID, err := uuid.Parse(req.SourceDocID)
		if err != nil {
			api.WriteError(w, api.NewValidationError("source_doc_id must be a valid UUID", "source_doc_id"))
			return
		}
		rule.SourceDocID = &sourceID
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID != uuid.Nil {
		rule.CreatedBy = &userID
	}

	created, err := h.rules.Create(r.Context(), rule)
	if err != nil {
		h.logger.Error("CreateRule failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	h.publishRuleEvent(r, "quorant.ai.JurisdictionRuleCreated", created)
	api.WriteJSON(w, http.StatusCreated, created)
}

// ListRules handles GET /api/v1/admin/jurisdiction-rules.
func (h *JurisdictionAdminHandler) ListRules(w http.ResponseWriter, r *http.Request) {
	jurisdiction := r.URL.Query().Get("jurisdiction")
	if jurisdiction == "" {
		api.WriteError(w, api.NewValidationError("jurisdiction query parameter is required", "jurisdiction"))
		return
	}

	page := api.ParsePageRequest(r)

	afterID, err := parseComplianceCursorID(page.Cursor)
	if err != nil {
		api.WriteError(w, api.NewValidationError("invalid cursor", "cursor"))
		return
	}

	rules, hasMore, err := h.rules.ListAllRules(r.Context(), jurisdiction, page.Limit, afterID)
	if err != nil {
		h.logger.Error("ListRules failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	var nextCursor string
	if hasMore && len(rules) > 0 {
		last := rules[len(rules)-1]
		nextCursor = api.EncodeCursor(map[string]string{"id": last.ID.String()})
	}

	api.WriteJSONWithMeta(w, http.StatusOK, rules, &api.Meta{
		Cursor:  nextCursor,
		HasMore: hasMore,
	})
}

// GetRule handles GET /api/v1/admin/jurisdiction-rules/{id}.
func (h *JurisdictionAdminHandler) GetRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	rule, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("GetRule failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if rule == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	api.WriteJSON(w, http.StatusOK, rule)
}

// UpdateRule handles PUT /api/v1/admin/jurisdiction-rules/{id}.
// Expires the existing rule and creates a new one with the updated values (immutable audit trail).
func (h *JurisdictionAdminHandler) UpdateRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	var req UpdateJurisdictionRuleRequest
	if err := api.ReadJSON(r, &req); err != nil {
		api.WriteError(w, err)
		return
	}

	if err := req.Validate(); err != nil {
		api.WriteError(w, api.NewValidationError(err.Error(), ""))
		return
	}

	existing, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("UpdateRule FindByID failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if existing == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	// Expire the existing rule.
	now := time.Now()
	existing.ExpirationDate = &now
	if _, err := h.rules.Update(r.Context(), existing); err != nil {
		h.logger.Error("UpdateRule expire failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		api.WriteError(w, api.NewValidationError("effective_date must be YYYY-MM-DD", "effective_date"))
		return
	}

	// Create the replacement rule with the same identity fields.
	replacement := &JurisdictionRule{
		Jurisdiction:     existing.Jurisdiction,
		RuleCategory:     existing.RuleCategory,
		RuleKey:          existing.RuleKey,
		ValueType:        existing.ValueType,
		Value:            req.Value,
		StatuteReference: req.StatuteReference,
		EffectiveDate:    effectiveDate,
		Notes:            req.Notes,
		SourceDocID:      existing.SourceDocID,
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID != uuid.Nil {
		replacement.CreatedBy = &userID
	}

	created, err := h.rules.Create(r.Context(), replacement)
	if err != nil {
		h.logger.Error("UpdateRule Create replacement failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	h.publishRuleEvent(r, "quorant.ai.JurisdictionRuleUpdated", created)
	api.WriteJSON(w, http.StatusOK, created)
}

// ExpireRule handles DELETE /api/v1/admin/jurisdiction-rules/{id}.
// Sets the expiration date to now, marking the rule as no longer active.
func (h *JurisdictionAdminHandler) ExpireRule(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		api.WriteError(w, api.NewValidationError("id must be a valid UUID", "id"))
		return
	}

	rule, err := h.rules.FindByID(r.Context(), id)
	if err != nil {
		h.logger.Error("ExpireRule FindByID failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}
	if rule == nil {
		api.WriteError(w, api.NewNotFoundError("jurisdiction rule not found"))
		return
	}

	now := time.Now()
	rule.ExpirationDate = &now

	updated, err := h.rules.Update(r.Context(), rule)
	if err != nil {
		h.logger.Error("ExpireRule Update failed", "error", err)
		api.WriteError(w, api.NewInternalError(err))
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// publishRuleEvent publishes a NATS event for jurisdiction rule changes.
func (h *JurisdictionAdminHandler) publishRuleEvent(r *http.Request, eventType string, rule *JurisdictionRule) {
	userID := middleware.UserIDFromContext(r.Context())
	payload, _ := json.Marshal(map[string]any{
		"rule_id":       rule.ID,
		"jurisdiction":  rule.Jurisdiction,
		"rule_category": rule.RuleCategory,
		"rule_key":      rule.RuleKey,
		"effective_date": rule.EffectiveDate.Format("2006-01-02"),
		"changed_by":    userID,
	})
	event := queue.NewBaseEvent(eventType, "jurisdiction_rule", rule.ID, uuid.Nil, payload)
	if err := h.publisher.Publish(r.Context(), event); err != nil {
		h.logger.Error("failed to publish "+eventType, "rule_id", rule.ID, "error", err)
	}
}
