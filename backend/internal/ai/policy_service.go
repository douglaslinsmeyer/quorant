package ai

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// PolicyService orchestrates policy engine operations.
type PolicyService struct {
	repo   PolicyRepository
	logger *slog.Logger
}

// NewPolicyService constructs a PolicyService with the given repository and logger.
func NewPolicyService(repo PolicyRepository, logger *slog.Logger) *PolicyService {
	return &PolicyService{repo: repo, logger: logger}
}

// ─── Governing Documents ──────────────────────────────────────────────────────

// RegisterGoverningDoc creates a new governing document record with indexing_status set to 'pending'.
func (s *PolicyService) RegisterGoverningDoc(ctx context.Context, orgID uuid.UUID, req RegisterGoverningDocRequest) (*GoverningDocument, error) {
	documentID, err := uuid.Parse(req.DocumentID)
	if err != nil {
		return nil, api.NewValidationError("document_id must be a valid UUID", "document_id")
	}
	if req.DocType == "" {
		return nil, api.NewValidationError("doc_type is required", "doc_type")
	}
	if req.Title == "" {
		return nil, api.NewValidationError("title is required", "title")
	}
	effectiveDate, err := time.Parse("2006-01-02", req.EffectiveDate)
	if err != nil {
		return nil, api.NewValidationError("effective_date must be in YYYY-MM-DD format", "effective_date")
	}

	doc := &GoverningDocument{
		OrgID:          orgID,
		DocumentID:     documentID,
		DocType:        req.DocType,
		Title:          req.Title,
		EffectiveDate:  effectiveDate,
		IndexingStatus: "pending",
	}

	return s.repo.CreateGoverningDoc(ctx, doc)
}

// ListGoverningDocs returns all governing documents for the given org.
func (s *PolicyService) ListGoverningDocs(ctx context.Context, orgID uuid.UUID) ([]GoverningDocument, error) {
	return s.repo.ListGoverningDocsByOrg(ctx, orgID)
}

// GetGoverningDoc returns a governing document by ID, or a 404 error if not found.
func (s *PolicyService) GetGoverningDoc(ctx context.Context, id uuid.UUID) (*GoverningDocument, error) {
	doc, err := s.repo.FindGoverningDocByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, api.NewNotFoundError("governing document not found")
	}
	return doc, nil
}

// ReindexGoverningDoc resets the indexing_status of a governing document to 'pending'.
func (s *PolicyService) ReindexGoverningDoc(ctx context.Context, id uuid.UUID) (*GoverningDocument, error) {
	doc, err := s.GetGoverningDoc(ctx, id)
	if err != nil {
		return nil, err
	}
	doc.IndexingStatus = "pending"
	doc.IndexedAt = nil
	doc.ChunkCount = nil
	doc.ExtractionCount = nil
	return s.repo.UpdateGoverningDoc(ctx, doc)
}

// UpdateGoverningDoc updates a governing document.
func (s *PolicyService) UpdateGoverningDoc(ctx context.Context, doc *GoverningDocument) (*GoverningDocument, error) {
	return s.repo.UpdateGoverningDoc(ctx, doc)
}

// ─── Policy Extractions ───────────────────────────────────────────────────────

// ListExtractions returns all policy extractions for the given org.
func (s *PolicyService) ListExtractions(ctx context.Context, orgID uuid.UUID) ([]PolicyExtraction, error) {
	return s.repo.ListExtractionsByOrg(ctx, orgID)
}

// ListActivePolicies returns only active (not superseded, approved or pending) extractions for the given org.
func (s *PolicyService) ListActivePolicies(ctx context.Context, orgID uuid.UUID) ([]PolicyExtraction, error) {
	return s.repo.ListActiveExtractionsByOrg(ctx, orgID)
}

// GetExtraction returns a policy extraction by ID, or a 404 error if not found.
func (s *PolicyService) GetExtraction(ctx context.Context, id uuid.UUID) (*PolicyExtraction, error) {
	e, err := s.repo.FindExtractionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, api.NewNotFoundError("policy extraction not found")
	}
	return e, nil
}

// ApproveExtraction sets the review_status of a policy extraction to 'approved'.
func (s *PolicyService) ApproveExtraction(ctx context.Context, id uuid.UUID, reviewedBy uuid.UUID) (*PolicyExtraction, error) {
	e, err := s.GetExtraction(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	e.ReviewStatus = "approved"
	e.ReviewedBy = &reviewedBy
	e.ReviewedAt = &now
	return s.repo.UpdateExtraction(ctx, e)
}

// RejectExtraction sets the review_status of a policy extraction to 'rejected'.
func (s *PolicyService) RejectExtraction(ctx context.Context, id uuid.UUID, reviewedBy uuid.UUID) (*PolicyExtraction, error) {
	e, err := s.GetExtraction(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	e.ReviewStatus = "rejected"
	e.ReviewedBy = &reviewedBy
	e.ReviewedAt = &now
	return s.repo.UpdateExtraction(ctx, e)
}

// ModifyExtraction sets the review_status to 'modified' and stores the human override.
func (s *PolicyService) ModifyExtraction(ctx context.Context, id uuid.UUID, override json.RawMessage, reviewedBy uuid.UUID) (*PolicyExtraction, error) {
	e, err := s.GetExtraction(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	e.ReviewStatus = "modified"
	e.HumanOverride = override
	e.ReviewedBy = &reviewedBy
	e.ReviewedAt = &now
	return s.repo.UpdateExtraction(ctx, e)
}

// GetActivePolicy returns the active policy extraction for the given org and policy key as a PolicyResult.
// Returns nil (not an error) when no active extraction exists.
func (s *PolicyService) GetActivePolicy(ctx context.Context, orgID uuid.UUID, policyKey string) (*PolicyResult, error) {
	extraction, err := s.repo.FindActiveExtraction(ctx, orgID, policyKey)
	if err != nil {
		return nil, err
	}
	if extraction == nil {
		return nil, nil
	}

	// Use the human override config if available, otherwise use the AI-extracted config.
	config := extraction.Config
	if len(extraction.HumanOverride) > 0 {
		config = extraction.HumanOverride
	}

	sourceSection := ""
	if extraction.SourceSection != nil {
		sourceSection = *extraction.SourceSection
	}

	return &PolicyResult{
		Config:         config,
		Confidence:     extraction.Confidence,
		ReviewStatus:   extraction.ReviewStatus,
		SourceSection:  sourceSection,
		RequiresReview: extraction.ReviewStatus == "pending",
	}, nil
}

// ─── Policy Resolutions ───────────────────────────────────────────────────────

// ListResolutions returns all policy resolutions for the given org.
func (s *PolicyService) ListResolutions(ctx context.Context, orgID uuid.UUID) ([]PolicyResolution, error) {
	return s.repo.ListResolutionsByOrg(ctx, orgID)
}

// GetResolution returns a policy resolution by ID, or a 404 error if not found.
func (s *PolicyService) GetResolution(ctx context.Context, id uuid.UUID) (*PolicyResolution, error) {
	r, err := s.repo.FindResolutionByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if r == nil {
		return nil, api.NewNotFoundError("policy resolution not found")
	}
	return r, nil
}

// DecideResolution records a human decision on a human-escalated resolution.
func (s *PolicyService) DecideResolution(ctx context.Context, id uuid.UUID, decision json.RawMessage, decidedBy uuid.UUID) (*PolicyResolution, error) {
	r, err := s.GetResolution(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	r.HumanDecision = decision
	r.DecidedBy = &decidedBy
	r.DecidedAt = &now
	return s.repo.UpdateResolution(ctx, r)
}
