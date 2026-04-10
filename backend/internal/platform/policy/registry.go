package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/ai"
)

// OrgJurisdictionLookup resolves the jurisdiction string for a given org.
type OrgJurisdictionLookup interface {
	GetJurisdiction(ctx context.Context, orgID uuid.UUID) (string, error)
}

// Registry holds all registered OperationDescriptors and provides trigger
// matching for document ingestion and concept extraction pipelines.
type Registry struct {
	mu          sync.RWMutex
	descriptors map[string]OperationDescriptor // keyed by category
	records     PolicyRecordRepository
	resolutions ResolutionRepository
	ai          ai.PolicyResolver
	orgLookup   OrgJurisdictionLookup
	logger      *slog.Logger
}

// NewRegistry constructs a Registry. All parameters are optional and may be
// nil; nil dependencies are acceptable for unit tests and deferred wiring.
func NewRegistry(
	records PolicyRecordRepository,
	resolutions ResolutionRepository,
	aiResolver ai.PolicyResolver,
	orgLookup OrgJurisdictionLookup,
	logger *slog.Logger,
) *Registry {
	return &Registry{
		descriptors: make(map[string]OperationDescriptor),
		records:     records,
		resolutions: resolutions,
		ai:          aiResolver,
		orgLookup:   orgLookup,
		logger:      logger,
	}
}

// lookupJurisdiction returns the jurisdiction for orgID, falling back to
// "DEFAULT" when the lookup is unavailable or returns an error.
func (r *Registry) lookupJurisdiction(ctx context.Context, orgID uuid.UUID) string {
	if r.orgLookup == nil {
		return "DEFAULT"
	}
	j, err := r.orgLookup.GetJurisdiction(ctx, orgID)
	if err != nil || j == "" {
		if r.logger != nil {
			r.logger.WarnContext(ctx, "jurisdiction lookup failed, using DEFAULT",
				"org_id", orgID,
				"error", err,
			)
		}
		return "DEFAULT"
	}
	return j
}

// Register adds an OperationDescriptor under the given category. Returns an
// error if a descriptor for that category has already been registered.
func (r *Registry) Register(category string, desc OperationDescriptor) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.descriptors[category]; exists {
		return fmt.Errorf("policy: category %q is already registered", category)
	}
	r.descriptors[category] = desc
	return nil
}

// FindTriggers returns all MatchedTrigger entries whose PolicySpec matches the
// given documentType (exact string match) or any of the provided concepts
// (case-insensitive). An empty documentType and nil/empty concepts returns
// an empty slice.
func (r *Registry) FindTriggers(documentType string, concepts []string) []MatchedTrigger {
	r.mu.RLock()
	defer r.mu.RUnlock()

	results := make([]MatchedTrigger, 0)
	for category, desc := range r.descriptors {
		for key, spec := range desc.Policies {
			if r.matchesTrigger(spec, documentType, concepts) {
				results = append(results, MatchedTrigger{
					Category: category,
					Key:      key,
					Spec:     spec,
				})
			}
		}
	}
	return results
}

// matchesTrigger reports whether the given spec matches the document type or
// any of the provided concepts.
func (r *Registry) matchesTrigger(spec PolicySpec, documentType string, concepts []string) bool {
	if documentType != "" {
		for _, dt := range spec.DocumentTypes {
			if dt == documentType {
				return true
			}
		}
	}

	for _, concept := range concepts {
		for _, specConcept := range spec.Concepts {
			if strings.EqualFold(concept, specConcept) {
				return true
			}
		}
	}

	return false
}

// Resolve executes the two-tier policy resolution pipeline for the given
// category. Tier 1 gathers applicable policy records from the database.
// Tier 2 sends them to the AI resolver for precedence reasoning. The result
// is persisted (when a resolutions repo is available) and returned.
func (r *Registry) Resolve(ctx context.Context, orgID uuid.UUID, unitID *uuid.UUID, category string) (*Resolution, error) {
	r.mu.RLock()
	desc, exists := r.descriptors[category]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("policy: category %q is not registered", category)
	}

	// Tier 1: gather applicable policy records.
	jurisdiction := r.lookupJurisdiction(ctx, orgID)

	records, err := r.records.GatherForResolution(ctx, category, jurisdiction, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("policy: gather records for %q: %w", category, err)
	}

	// Build policy ID slice for audit trail.
	policyIDs := make([]uuid.UUID, len(records))
	for i, rec := range records {
		policyIDs[i] = rec.ID
	}

	// Tier 2: AI resolution.
	var (
		ruling       json.RawMessage
		reasoning    string
		confidence   float64
		modelID      string
		status       string
		reviewStatus string
	)

	ruling, reasoning, confidence, modelID, err = r.callTier2(ctx, orgID, desc, records)
	if err != nil {
		// AI unavailable -- hold for human review.
		if r.logger != nil {
			r.logger.WarnContext(ctx, "tier 2 AI call failed, holding resolution",
				"category", category,
				"org_id", orgID,
				"error", err,
			)
		}
		status = "held"
		reviewStatus = "ai_unavailable"
		confidence = 0
		ruling = nil
		reasoning = ""
	} else {
		threshold := desc.DefaultThreshold
		if threshold == 0 {
			threshold = 0.80
		}

		if confidence < threshold {
			status = "held"
			reviewStatus = "pending_review"
		} else {
			status = "approved"
			reviewStatus = "auto_approved"
		}
	}

	resID := uuid.New()
	res := &Resolution{
		ID:         resID,
		Status:     status,
		Ruling:     ruling,
		Reasoning:  reasoning,
		Confidence: confidence,
	}

	// Build source policy references from the gathered records.
	refs := make([]PolicyReference, len(records))
	for i, rec := range records {
		refs[i] = PolicyReference{
			ID:           rec.ID,
			Scope:        rec.Scope,
			Category:     rec.Category,
			Key:          rec.Key,
			PriorityHint: rec.PriorityHint,
			StatuteRef:   rec.StatuteRef,
		}
	}
	res.SourcePolicies = refs

	// Persist if resolutions repo is available.
	if r.resolutions != nil {
		record := &ResolutionRecord{
			ID:             resID,
			OrgID:          orgID,
			UnitID:         unitID,
			Category:       category,
			InputPolicyIDs: policyIDs,
			Ruling:         ruling,
			Reasoning:      reasoning,
			Confidence:     confidence,
			ModelID:        modelID,
			ReviewStatus:   reviewStatus,
		}
		if _, persistErr := r.resolutions.CreateResolution(ctx, record); persistErr != nil {
			if r.logger != nil {
				r.logger.ErrorContext(ctx, "failed to persist resolution",
					"resolution_id", resID,
					"error", persistErr,
				)
			}
		}
	}

	// Invoke OnHold callback if held.
	if res.Held() && desc.OnHold != nil {
		if holdErr := desc.OnHold(ctx, res); holdErr != nil {
			if r.logger != nil {
				r.logger.ErrorContext(ctx, "OnHold callback failed",
					"category", category,
					"resolution_id", resID,
					"error", holdErr,
				)
			}
		}
	}

	return res, nil
}

// callTier2 marshals the gathered records and sends them to the AI resolver.
// It returns the ruling, reasoning, confidence, model ID, and any error.
func (r *Registry) callTier2(ctx context.Context, orgID uuid.UUID, desc OperationDescriptor, records []PolicyRecord) (json.RawMessage, string, float64, string, error) {
	if r.ai == nil {
		return nil, "", 0, "", fmt.Errorf("policy: AI resolver is not configured")
	}

	recordsJSON, err := json.Marshal(records)
	if err != nil {
		return nil, "", 0, "", fmt.Errorf("policy: marshal records: %w", err)
	}

	query := strings.ReplaceAll(desc.PromptTemplate, "{{.Policies}}", string(recordsJSON))

	qctx := ai.QueryContext{
		Module:       "policy",
		ResourceType: desc.Category,
	}

	result, err := r.ai.QueryPolicy(ctx, orgID, query, qctx)
	if err != nil {
		return nil, "", 0, "", err
	}

	return result.Resolution, result.Reasoning, result.Confidence, "ai-model", nil
}
