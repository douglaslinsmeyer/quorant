package estoppel_test

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/audit"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/quorant/quorant/internal/platform/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// In-memory mock repository
// ---------------------------------------------------------------------------

type mockRepo struct {
	mu           sync.Mutex
	requests     map[uuid.UUID]*estoppel.EstoppelRequest
	certificates map[uuid.UUID]*estoppel.EstoppelCertificate
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		requests:     make(map[uuid.UUID]*estoppel.EstoppelRequest),
		certificates: make(map[uuid.UUID]*estoppel.EstoppelCertificate),
	}
}

func (r *mockRepo) CreateRequest(_ context.Context, req *estoppel.EstoppelRequest) (*estoppel.EstoppelRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *req
	cp.ID = uuid.New()
	cp.CreatedAt = time.Now()
	cp.UpdatedAt = time.Now()
	r.requests[cp.ID] = &cp
	return &cp, nil
}

func (r *mockRepo) FindRequestByID(_ context.Context, id uuid.UUID) (*estoppel.EstoppelRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return nil, nil
	}
	cp := *req
	return &cp, nil
}

func (r *mockRepo) ListRequestsByOrg(_ context.Context, orgID uuid.UUID, status *string, limit int, afterID *uuid.UUID) ([]estoppel.EstoppelRequest, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []estoppel.EstoppelRequest
	for _, req := range r.requests {
		if req.OrgID != orgID {
			continue
		}
		if status != nil && req.Status != *status {
			continue
		}
		result = append(result, *req)
	}
	return result, false, nil
}

func (r *mockRepo) UpdateRequestStatus(_ context.Context, id uuid.UUID, status string) (*estoppel.EstoppelRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return nil, nil
	}
	req.Status = status
	req.UpdatedAt = time.Now()
	cp := *req
	return &cp, nil
}

func (r *mockRepo) UpdateRequestNarratives(_ context.Context, id uuid.UUID, narrativeSections []byte) (*estoppel.EstoppelRequest, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.requests[id]
	if !ok {
		return nil, nil
	}
	req.UpdatedAt = time.Now()
	cp := *req
	return &cp, nil
}

func (r *mockRepo) CreateCertificate(_ context.Context, c *estoppel.EstoppelCertificate) (*estoppel.EstoppelCertificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cp := *c
	cp.ID = uuid.New()
	cp.CreatedAt = time.Now()
	r.certificates[cp.ID] = &cp
	return &cp, nil
}

func (r *mockRepo) FindCertificateByID(_ context.Context, id uuid.UUID) (*estoppel.EstoppelCertificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cert, ok := r.certificates[id]
	if !ok {
		return nil, nil
	}
	cp := *cert
	return &cp, nil
}

func (r *mockRepo) FindCertificateByRequestID(_ context.Context, requestID uuid.UUID) (*estoppel.EstoppelCertificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, cert := range r.certificates {
		if cert.RequestID == requestID {
			cp := *cert
			return &cp, nil
		}
	}
	return nil, nil
}

func (r *mockRepo) ListCertificatesByOrg(_ context.Context, orgID uuid.UUID) ([]estoppel.EstoppelCertificate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var result []estoppel.EstoppelCertificate
	for _, cert := range r.certificates {
		if cert.OrgID == orgID {
			result = append(result, *cert)
		}
	}
	return result, nil
}

// ---------------------------------------------------------------------------
// Mock CertificateGenerator
// ---------------------------------------------------------------------------

type mockCertificateGenerator struct{}

func (g *mockCertificateGenerator) GenerateEstoppel(_ *estoppel.AggregatedData, _ *estoppel.EstoppelRules) ([]byte, error) {
	return []byte("%PDF-1.4 mock"), nil
}

func (g *mockCertificateGenerator) GenerateLenderQuestionnaire(_ *estoppel.AggregatedData, _ *estoppel.EstoppelRules) ([]byte, error) {
	return []byte("%PDF-1.4 mock"), nil
}

// ---------------------------------------------------------------------------
// Test helper: newTestService
// ---------------------------------------------------------------------------

func newTestService() *estoppel.EstoppelService {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	return estoppel.NewEstoppelService(
		newMockRepo(),
		&mockFinancialProvider{
			snapshot: &estoppel.FinancialSnapshot{
				RegularAssessmentCents: 25000,
				AssessmentFrequency:    "monthly",
				TotalDelinquentCents:   0,
			},
		},
		&mockComplianceProvider{
			snapshot: &estoppel.ComplianceSnapshot{},
		},
		&mockPropertyProvider{
			snapshot: &estoppel.PropertySnapshot{
				UnitNumber: "1A",
				Address:    "1 Test Ave",
			},
		},
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
}

func defaultRules() *estoppel.EstoppelRules {
	return &estoppel.EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
	}
}

func validCreateDTO() estoppel.CreateEstoppelRequestDTO {
	return estoppel.CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "Alice Smith",
		RequestorEmail:  "alice@titleco.com",
		RequestorPhone:  "555-0100",
		RequestorCompany: "Title Co",
		PropertyAddress: "123 Main St",
		OwnerName:       "Bob Jones",
		RushRequested:   false,
	}
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestCreateRequest_Success(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()
	createdBy := uuid.New()
	rules := defaultRules()
	dto := validCreateDTO()

	req, err := svc.CreateRequest(context.Background(), orgID, dto, rules, createdBy)

	require.NoError(t, err)
	require.NotNil(t, req)
	assert.Equal(t, "submitted", req.Status)
	assert.Equal(t, rules.StandardFeeCents, req.FeeCents)
	assert.Equal(t, int64(0), req.RushFeeCents)
	assert.Equal(t, int64(0), req.DelinquentSurchargeCents)
	assert.Equal(t, rules.StandardFeeCents, req.TotalFeeCents)
	assert.False(t, req.DeadlineAt.IsZero())
	assert.True(t, req.DeadlineAt.After(time.Now()))
	assert.Equal(t, orgID, req.OrgID)
	assert.Equal(t, createdBy, req.CreatedBy)
}

func TestCreateRequest_ValidationFailure(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()
	createdBy := uuid.New()
	rules := defaultRules()

	// Empty DTO — missing required fields.
	dto := estoppel.CreateEstoppelRequestDTO{}

	req, err := svc.CreateRequest(context.Background(), orgID, dto, rules, createdBy)

	assert.Nil(t, req)
	require.Error(t, err)
}

func TestCreateRequest_WithRushFee(t *testing.T) {
	rushDays := 3
	svc := newTestService()
	orgID := uuid.New()
	rules := &estoppel.EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
		RushFeeCents:                   15000,
		RushTurnaroundBusinessDays:     &rushDays,
	}

	dto := validCreateDTO()
	dto.RushRequested = true

	req, err := svc.CreateRequest(context.Background(), orgID, dto, rules, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, int64(15000), req.RushFeeCents)
	assert.Equal(t, rules.StandardFeeCents+rules.RushFeeCents, req.TotalFeeCents)
}

func TestCreateRequest_WithDelinquency(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := estoppel.NewEstoppelService(
		newMockRepo(),
		&mockFinancialProvider{
			snapshot: &estoppel.FinancialSnapshot{
				TotalDelinquentCents: 50000, // delinquent
			},
		},
		&mockComplianceProvider{snapshot: &estoppel.ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &estoppel.PropertySnapshot{UnitNumber: "1A"}},
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)

	rules := &estoppel.EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
		DelinquentSurchargeCents:       10000,
	}

	req, err := svc.CreateRequest(context.Background(), uuid.New(), validCreateDTO(), rules, uuid.New())

	require.NoError(t, err)
	assert.Equal(t, int64(10000), req.DelinquentSurchargeCents)
	assert.Equal(t, rules.StandardFeeCents+rules.DelinquentSurchargeCents, req.TotalFeeCents)
}

func TestAggregateData_Success(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()
	rules := defaultRules()

	created, err := svc.CreateRequest(context.Background(), orgID, validCreateDTO(), rules, uuid.New())
	require.NoError(t, err)

	data, err := svc.AggregateData(context.Background(), created)

	require.NoError(t, err)
	require.NotNil(t, data)
	assert.NotEmpty(t, data.Property.UnitNumber)
	assert.NotZero(t, data.Financial.RegularAssessmentCents)
	assert.NotNil(t, data.Narratives)
	assert.False(t, data.AsOfTime.IsZero())
}

func TestApproveRequest_Success(t *testing.T) {
	orgID := uuid.New()

	// Create → must be at "submitted", approve transitions to "approved" via "manager_review".
	// According to domain validTransitions:
	//   submitted → data_aggregation → manager_review → approved
	// We need to walk the request through the states to reach manager_review first.
	// However, the service only has ApproveRequest that transitions to "approved",
	// which requires current status to be "manager_review".
	// For the test, we'll manually update the status via the mock repo.

	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	publisher := queue.NewInMemoryPublisher()
	svc2 := estoppel.NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &estoppel.FinancialSnapshot{}},
		&mockComplianceProvider{snapshot: &estoppel.ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &estoppel.PropertySnapshot{UnitNumber: "1A"}},
		estoppel.NewNoopNarrativeGenerator(),
		&mockCertificateGenerator{},
		audit.NewNoopAuditor(),
		publisher,
		logger,
	)

	created, err := svc2.CreateRequest(context.Background(), orgID, validCreateDTO(), defaultRules(), uuid.New())
	require.NoError(t, err)

	// Walk through required transitions: submitted → data_aggregation → manager_review
	_, err = repo.UpdateRequestStatus(context.Background(), created.ID, "data_aggregation")
	require.NoError(t, err)
	_, err = repo.UpdateRequestStatus(context.Background(), created.ID, "manager_review")
	require.NoError(t, err)

	dto := estoppel.ApproveRequestDTO{SignerTitle: "HOA Manager"}
	approved, err := svc2.ApproveRequest(context.Background(), created.ID, dto, uuid.New())

	require.NoError(t, err)
	require.NotNil(t, approved)
	assert.Equal(t, "approved", approved.Status)

	// Verify event was published.
	events := publisher.Events()
	var foundApproved bool
	for _, e := range events {
		if e.EventType() == "estoppel_request.approved" {
			foundApproved = true
		}
	}
	assert.True(t, foundApproved, "expected estoppel_request.approved event")
}

func TestApproveRequest_InvalidTransition(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()

	// "submitted" → "approved" is not a valid transition.
	created, err := svc.CreateRequest(context.Background(), orgID, validCreateDTO(), defaultRules(), uuid.New())
	require.NoError(t, err)

	dto := estoppel.ApproveRequestDTO{SignerTitle: "Manager"}
	_, err = svc.ApproveRequest(context.Background(), created.ID, dto, uuid.New())

	require.Error(t, err)
}

func TestRejectRequest_Success(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()

	created, err := svc.CreateRequest(context.Background(), orgID, validCreateDTO(), defaultRules(), uuid.New())
	require.NoError(t, err)

	dto := estoppel.RejectRequestDTO{Reason: "requestor did not provide ID"}
	rejected, err := svc.RejectRequest(context.Background(), created.ID, dto, uuid.New())

	require.NoError(t, err)
	require.NotNil(t, rejected)
	assert.Equal(t, "cancelled", rejected.Status)
}

func TestRejectRequest_ValidationFailure(t *testing.T) {
	svc := newTestService()
	orgID := uuid.New()

	created, err := svc.CreateRequest(context.Background(), orgID, validCreateDTO(), defaultRules(), uuid.New())
	require.NoError(t, err)

	// Missing reason.
	dto := estoppel.RejectRequestDTO{}
	_, err = svc.RejectRequest(context.Background(), created.ID, dto, uuid.New())

	require.Error(t, err)
}

func TestGetRequest_NotFound(t *testing.T) {
	svc := newTestService()

	req, err := svc.GetRequest(context.Background(), uuid.New())
	assert.Nil(t, req)
	require.Error(t, err)
}

func TestListRequests_Empty(t *testing.T) {
	svc := newTestService()

	requests, hasMore, err := svc.ListRequests(context.Background(), uuid.New(), nil, 20, nil)
	require.NoError(t, err)
	assert.False(t, hasMore)
	assert.Empty(t, requests)
}
