package estoppel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/platform/api"
	"github.com/quorant/quorant/internal/platform/queue"
	"golang.org/x/sync/errgroup"
)


// EstoppelService orchestrates all business logic for the Estoppel module:
// request intake, data aggregation, manager review, PDF generation, and delivery.
type EstoppelService struct {
	repo         EstoppelRepository
	financial    FinancialDataProvider
	compliance   ComplianceDataProvider
	property     PropertyDataProvider
	narrative    NarrativeGenerator
	generator    CertificateGenerator
	docUploader  DocumentUploader
	docDownloader DocumentDownloader
	auditor      audit.Auditor
	publisher    queue.Publisher
	logger       *slog.Logger
}

// NewEstoppelService creates a new EstoppelService wired to all required
// dependencies. docUploader and docDownloader may be nil; when nil, PDF bytes
// are generated but not persisted to document storage.
func NewEstoppelService(
	repo EstoppelRepository,
	financial FinancialDataProvider,
	compliance ComplianceDataProvider,
	property PropertyDataProvider,
	narrative NarrativeGenerator,
	generator CertificateGenerator,
	docUploader DocumentUploader,
	docDownloader DocumentDownloader,
	auditor audit.Auditor,
	publisher queue.Publisher,
	logger *slog.Logger,
) *EstoppelService {
	return &EstoppelService{
		repo:          repo,
		financial:     financial,
		compliance:    compliance,
		property:      property,
		narrative:     narrative,
		generator:     generator,
		docUploader:   docUploader,
		docDownloader: docDownloader,
		auditor:       auditor,
		publisher:     publisher,
		logger:        logger,
	}
}

// CreateRequest validates the DTO, checks delinquency, computes fees and
// deadline, persists a new estoppel request, emits an audit entry, and
// publishes an EventRequestCreated event.
func (s *EstoppelService) CreateRequest(
	ctx context.Context,
	orgID uuid.UUID,
	dto CreateEstoppelRequestDTO,
	rules *EstoppelRules,
	createdBy uuid.UUID,
) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	// Check delinquency via financial provider.
	delinquent := false
	fin, err := s.financial.GetUnitFinancialSnapshot(ctx, orgID, dto.UnitID)
	if err != nil {
		s.logger.Warn("could not fetch financial snapshot for delinquency check",
			"org_id", orgID, "unit_id", dto.UnitID, "error", err)
		// Non-fatal: proceed without delinquency surcharge.
	} else if fin != nil && fin.TotalDelinquentCents > 0 {
		delinquent = true
	}

	fees := CalculateFees(rules, dto.RushRequested, delinquent)
	deadline := CalculateDeadline(rules, dto.RushRequested, time.Now())

	req := &EstoppelRequest{
		OrgID:                    orgID,
		UnitID:                   dto.UnitID,
		RequestType:              dto.RequestType,
		RequestorType:            dto.RequestorType,
		RequestorName:            dto.RequestorName,
		RequestorEmail:           dto.RequestorEmail,
		RequestorPhone:           dto.RequestorPhone,
		RequestorCompany:         dto.RequestorCompany,
		PropertyAddress:          dto.PropertyAddress,
		OwnerName:                dto.OwnerName,
		ClosingDate:              dto.ClosingDate,
		RushRequested:            dto.RushRequested,
		Status:                   "submitted",
		FeeCents:                 fees.FeeCents,
		RushFeeCents:             fees.RushFeeCents,
		DelinquentSurchargeCents: fees.DelinquentSurchargeCents,
		TotalFeeCents:            fees.TotalFeeCents,
		DeadlineAt:               deadline,
		Metadata:                 dto.Metadata,
		CreatedBy:                createdBy,
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}

	created, err := s.repo.CreateRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("creating estoppel request: %w", err)
	}

	// Audit.
	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      createdBy,
		Action:       "estoppel_request.created",
		ResourceType: "estoppel_request",
		ResourceID:   created.ID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	// Publish event.
	evt := newEstoppelEvent(EventRequestCreated, created.ID, orgID, map[string]any{
		"request_type": created.RequestType,
		"unit_id":      created.UnitID,
	})
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Warn("failed to publish estoppel_request.created event", "request_id", created.ID, "error", err)
	}

	return created, nil
}

// AggregateData queries all three data providers in parallel, then generates
// narrative sections. The returned AggregatedData is ready for review and PDF
// generation.
func (s *EstoppelService) AggregateData(ctx context.Context, req *EstoppelRequest) (*AggregatedData, error) {
	var (
		finSnap  *FinancialSnapshot
		compSnap *ComplianceSnapshot
		propSnap *PropertySnapshot
	)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		snap, err := s.financial.GetUnitFinancialSnapshot(egCtx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("financial snapshot: %w", err)
		}
		finSnap = snap
		return nil
	})

	eg.Go(func() error {
		snap, err := s.compliance.GetUnitComplianceSnapshot(egCtx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("compliance snapshot: %w", err)
		}
		compSnap = snap
		return nil
	})

	eg.Go(func() error {
		snap, err := s.property.GetPropertySnapshot(egCtx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("property snapshot: %w", err)
		}
		propSnap = snap
		return nil
	})

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	data := &AggregatedData{
		AsOfTime: time.Now(),
	}
	if finSnap != nil {
		data.Financial = *finSnap
	}
	if compSnap != nil {
		data.Compliance = *compSnap
	}
	if propSnap != nil {
		data.Property = *propSnap
	}

	// Generate narratives.
	narratives, err := s.narrative.GenerateNarratives(ctx, req.OrgID, data)
	if err != nil {
		s.logger.Warn("narrative generation failed, proceeding without narratives",
			"request_id", req.ID, "error", err)
	} else {
		data.Narratives = narratives
	}

	// Publish aggregation event.
	evt := newEstoppelEvent(EventDataAggregated, req.ID, req.OrgID, nil)
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Warn("failed to publish estoppel_request.data_aggregated event",
			"request_id", req.ID, "error", err)
	}

	return data, nil
}

// GetRequest returns the estoppel request with the given id, or a 404 error if
// not found.
func (s *EstoppelService) GetRequest(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error) {
	req, err := s.repo.FindRequestByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding estoppel request: %w", err)
	}
	if req == nil {
		return nil, api.NewNotFoundError(fmt.Sprintf("estoppel request %s not found", id))
	}
	return req, nil
}

// ListRequests returns paginated requests for the given org, optionally
// filtered by status.
func (s *EstoppelService) ListRequests(
	ctx context.Context,
	orgID uuid.UUID,
	status *string,
	limit int,
	afterID *uuid.UUID,
) ([]EstoppelRequest, bool, error) {
	return s.repo.ListRequestsByOrg(ctx, orgID, status, limit, afterID)
}

// ApproveRequest validates the status transition, updates the request to
// "approved", emits an audit entry, and publishes EventRequestApproved.
func (s *EstoppelService) ApproveRequest(
	ctx context.Context,
	requestID uuid.UUID,
	dto ApproveRequestDTO,
	signedBy uuid.UUID,
) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	if !IsValidTransition(req.Status, "approved") {
		return nil, api.NewValidationError(
			fmt.Sprintf("cannot approve request in status %q", req.Status), "status",
		)
	}

	updated, err := s.repo.UpdateRequestStatus(ctx, requestID, "approved")
	if err != nil {
		return nil, fmt.Errorf("updating request status to approved: %w", err)
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        req.OrgID,
		ActorID:      signedBy,
		Action:       "estoppel_request.approved",
		ResourceType: "estoppel_request",
		ResourceID:   requestID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	evt := newEstoppelEvent(EventRequestApproved, requestID, req.OrgID, map[string]any{
		"signer_title": dto.SignerTitle,
		"signed_by":    signedBy,
	})
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Warn("failed to publish estoppel_request.approved event",
			"request_id", requestID, "error", err)
	}

	return updated, nil
}

// RejectRequest validates the status transition, cancels the request, and
// publishes EventRequestRejected.
func (s *EstoppelService) RejectRequest(
	ctx context.Context,
	requestID uuid.UUID,
	dto RejectRequestDTO,
	rejectedBy uuid.UUID,
) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	if !IsValidTransition(req.Status, "cancelled") {
		return nil, api.NewValidationError(
			fmt.Sprintf("cannot cancel request in status %q", req.Status), "status",
		)
	}

	updated, err := s.repo.UpdateRequestStatus(ctx, requestID, "cancelled")
	if err != nil {
		return nil, fmt.Errorf("updating request status to cancelled: %w", err)
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        req.OrgID,
		ActorID:      rejectedBy,
		Action:       "estoppel_request.rejected",
		ResourceType: "estoppel_request",
		ResourceID:   requestID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	evt := newEstoppelEvent(EventRequestRejected, requestID, req.OrgID, map[string]any{
		"reason":      dto.Reason,
		"rejected_by": rejectedBy,
	})
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Warn("failed to publish estoppel_request.rejected event",
			"request_id", requestID, "error", err)
	}

	return updated, nil
}

// GenerateCertificate produces the PDF, freezes the data snapshot as JSON,
// and persists a new EstoppelCertificate record linked to the request.
func (s *EstoppelService) GenerateCertificate(
	ctx context.Context,
	requestID uuid.UUID,
	data *AggregatedData,
	rules *EstoppelRules,
	signedBy uuid.UUID,
	signerTitle string,
) (*EstoppelCertificate, error) {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	// Generate PDF bytes.
	var pdfBytes []byte
	if req.RequestType == "lender_questionnaire" {
		pdfBytes, err = s.generator.GenerateLenderQuestionnaire(data, rules)
	} else {
		pdfBytes, err = s.generator.GenerateEstoppel(data, rules)
	}
	if err != nil {
		return nil, fmt.Errorf("generating PDF: %w", err)
	}

	// Upload PDF to document storage when an uploader is wired in.
	var docID *uuid.UUID
	if s.docUploader != nil {
		fileName := fmt.Sprintf("estoppel-%s.pdf", requestID)
		title := fmt.Sprintf("Estoppel Certificate - %s", req.OwnerName)
		id, uploadErr := s.docUploader.UploadFromBytes(ctx, req.OrgID, title, fileName, "application/pdf", pdfBytes, signedBy)
		if uploadErr != nil {
			return nil, fmt.Errorf("uploading PDF: %w", uploadErr)
		}
		docID = &id
	}

	// Freeze the data snapshot.
	snapshotJSON, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshalling data snapshot: %w", err)
	}

	// Freeze narrative sections.
	var narrativeJSON json.RawMessage
	if data.Narratives != nil {
		if b, mErr := json.Marshal(data.Narratives); mErr == nil {
			narrativeJSON = b
		}
	}

	now := time.Now()

	// Compute ExpiresAt from the effective period rules.
	var expiresAt *time.Time
	if rules.EffectivePeriodDays != nil && *rules.EffectivePeriodDays > 0 {
		exp := now.AddDate(0, 0, *rules.EffectivePeriodDays)
		expiresAt = &exp
	}

	cert := &EstoppelCertificate{
		RequestID:         requestID,
		OrgID:             req.OrgID,
		UnitID:            req.UnitID,
		DocumentID:        docID,
		Jurisdiction:      data.Property.OrgState,
		EffectiveDate:     now,
		ExpiresAt:         expiresAt,
		DataSnapshot:      json.RawMessage(snapshotJSON),
		NarrativeSections: narrativeJSON,
		SignedBy:          signedBy,
		SignedAt:          now,
		SignerTitle:       signerTitle,
		TemplateVersion:   "1.0",
	}

	created, err := s.repo.CreateCertificate(ctx, cert)
	if err != nil {
		return nil, fmt.Errorf("creating certificate record: %w", err)
	}

	evt := newEstoppelEvent(EventCertificateGenerated, requestID, req.OrgID, map[string]any{
		"certificate_id": created.ID,
	})
	if err := s.publisher.Publish(ctx, evt); err != nil {
		s.logger.Warn("failed to publish certificate_generated event",
			"request_id", requestID, "error", err)
	}

	return created, nil
}
