package policy

import (
	"context"
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
