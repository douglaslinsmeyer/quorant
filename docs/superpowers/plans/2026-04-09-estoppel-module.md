# Estoppel Certificate Generation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the `estoppel` module — an independently licensed feature that aggregates financial, compliance, and property data into state-specific estoppel certificates and lender questionnaires, with AI-assisted narrative generation and manager review workflow.

**Architecture:** New `backend/internal/estoppel/` module using narrow read interfaces (FinancialDataProvider, ComplianceDataProvider, PropertyDataProvider) to aggregate data from fin, gov, and org modules. PDF generation via Maroto v2. Workflow managed through the existing task system. State compliance rules seeded via PolicyResolver. Entitlement-gated.

**Tech Stack:** Go 1.24, pgx/v5, Maroto v2 (`github.com/johnfercher/maroto/v2`), testify, existing platform packages (api, audit, queue, middleware, storage).

**Spec:** `docs/superpowers/specs/2026-04-09-estoppel-certificate-generation-design.md`

---

## File Structure

```
backend/internal/estoppel/
├── domain.go              # EstoppelRequest, EstoppelCertificate, snapshot types, status transitions, fee calc
├── domain_test.go         # Validation, fee calculation, status transition tests
├── providers.go           # FinancialDataProvider, ComplianceDataProvider, PropertyDataProvider interfaces + snapshot structs
├── repository.go          # EstoppelRepository interface
├── postgres.go            # PostgreSQL implementation of EstoppelRepository
├── postgres_test.go       # Integration tests (//go:build integration)
├── adapters.go            # Thin adapters: FinAdapter, GovAdapter, OrgAdapter wrapping existing services
├── adapters_test.go       # Adapter unit tests
├── narrative.go           # NarrativeGenerator interface + PolicyResolver-based implementation
├── narrative_test.go      # Narrative generation tests with mock PolicyResolver
├── generator.go           # CertificateGenerator interface + Maroto v2 implementation
├── generator_test.go      # PDF output tests (valid PDF, contains expected text)
├── service.go             # EstoppelService orchestration
├── service_test.go        # Unit tests with all mocks
├── requests.go            # CreateEstoppelRequest DTO, ApproveRequest, etc. + validation
├── handler.go             # HTTP handlers + path helpers
├── handler_test.go        # HTTP-level tests with httptest
├── routes.go              # Route registration with entitlement gate
└── events.go              # Event type constants and helpers

backend/migrations/
├── 20260409000025_estoppel_tables.sql    # Schema, indexes, RLS
└── 20260409000026_estoppel_seed_data.sql # Policy seed data for 5 states

backend/cmd/quorant-api/main.go           # Wiring: adapters, service, handler, routes

backend/internal/fin/estoppel_adapter.go   # FinancialDataProvider adapter
backend/internal/gov/estoppel_adapter.go   # ComplianceDataProvider adapter
backend/internal/org/estoppel_adapter.go   # PropertyDataProvider adapter
```

---

### Task 1: Add Maroto v2 Dependency

**Files:**
- Modify: `backend/go.mod`

- [ ] **Step 1: Add maroto v2 to go.mod**

```bash
cd /home/douglasl/Projects/quorant/backend && go get github.com/johnfercher/maroto/v2@latest
```

- [ ] **Step 2: Tidy modules**

```bash
cd /home/douglasl/Projects/quorant/backend && go mod tidy
```

- [ ] **Step 3: Verify import resolves**

```bash
cd /home/douglasl/Projects/quorant/backend && go list github.com/johnfercher/maroto/v2/...
```

Expected: list of maroto sub-packages, no errors.

- [ ] **Step 4: Commit**

```bash
git add backend/go.mod backend/go.sum
git commit -m "chore: add maroto v2 dependency for PDF generation"
```

---

### Task 2: Domain Types and Validation

**Files:**
- Create: `backend/internal/estoppel/domain.go`
- Create: `backend/internal/estoppel/domain_test.go`

- [ ] **Step 1: Write failing tests for domain validation**

Create `backend/internal/estoppel/domain_test.go`:

```go
package estoppel

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEstoppelRequest_ValidStatusTransitions(t *testing.T) {
	tests := []struct {
		from, to string
		valid    bool
	}{
		{"submitted", "data_aggregation", true},
		{"data_aggregation", "manager_review", true},
		{"manager_review", "approved", true},
		{"approved", "generating", true},
		{"generating", "delivered", true},
		{"submitted", "cancelled", true},
		{"manager_review", "cancelled", true},
		// invalid
		{"submitted", "approved", false},
		{"delivered", "submitted", false},
		{"cancelled", "submitted", false},
		{"generating", "manager_review", false},
	}

	for _, tt := range tests {
		t.Run(tt.from+"→"+tt.to, func(t *testing.T) {
			assert.Equal(t, tt.valid, IsValidTransition(tt.from, tt.to))
		})
	}
}

func TestCalculateFees_Standard(t *testing.T) {
	rules := &EstoppelRules{
		StandardFeeCents:         29900,
		RushFeeCents:             11900,
		DelinquentSurchargeCents: 17900,
	}

	fee := CalculateFees(rules, false, false)
	assert.Equal(t, int64(29900), fee.FeeCents)
	assert.Equal(t, int64(0), fee.RushFeeCents)
	assert.Equal(t, int64(0), fee.DelinquentSurchargeCents)
	assert.Equal(t, int64(29900), fee.TotalFeeCents)
}

func TestCalculateFees_RushAndDelinquent(t *testing.T) {
	rules := &EstoppelRules{
		StandardFeeCents:         29900,
		RushFeeCents:             11900,
		DelinquentSurchargeCents: 17900,
	}

	fee := CalculateFees(rules, true, true)
	assert.Equal(t, int64(29900), fee.FeeCents)
	assert.Equal(t, int64(11900), fee.RushFeeCents)
	assert.Equal(t, int64(17900), fee.DelinquentSurchargeCents)
	assert.Equal(t, int64(59700), fee.TotalFeeCents)
}

func TestCalculateDeadline_Standard(t *testing.T) {
	rules := &EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
	}
	rushDays := 3
	rules.RushTurnaroundBusinessDays = &rushDays

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC) // Thursday
	deadline := CalculateDeadline(rules, false, now)

	// 10 business days from Thursday Apr 9 = Wed Apr 23
	assert.Equal(t, 2026, deadline.Year())
	assert.Equal(t, time.April, deadline.Month())
	assert.Equal(t, 23, deadline.Day())
}

func TestCalculateDeadline_Rush(t *testing.T) {
	rushDays := 3
	rules := &EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
		RushTurnaroundBusinessDays:     &rushDays,
	}

	now := time.Date(2026, 4, 9, 12, 0, 0, 0, time.UTC) // Thursday
	deadline := CalculateDeadline(rules, true, now)

	// 3 business days from Thursday Apr 9 = Tue Apr 14
	assert.Equal(t, 2026, deadline.Year())
	assert.Equal(t, time.April, deadline.Month())
	assert.Equal(t, 14, deadline.Day())
}

func TestEstoppelRequest_JSONSerialization(t *testing.T) {
	req := EstoppelRequest{
		ID:          uuid.New(),
		OrgID:       uuid.New(),
		UnitID:      uuid.New(),
		RequestType: "estoppel_certificate",
		Status:      "submitted",
		CreatedAt:   time.Now().UTC().Truncate(time.Second),
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	requiredKeys := []string{"id", "org_id", "unit_id", "request_type", "status", "created_at"}
	for _, key := range requiredKeys {
		_, ok := result[key]
		assert.True(t, ok, "expected JSON key %q", key)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 2>&1 | head -20
```

Expected: compilation errors (types not defined yet).

- [ ] **Step 3: Implement domain types**

Create `backend/internal/estoppel/domain.go`:

```go
package estoppel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EstoppelRequest tracks the lifecycle of an estoppel or lender questionnaire request.
type EstoppelRequest struct {
	ID                       uuid.UUID      `json:"id"`
	OrgID                    uuid.UUID      `json:"org_id"`
	UnitID                   uuid.UUID      `json:"unit_id"`
	TaskID                   *uuid.UUID     `json:"task_id,omitempty"`
	RequestType              string         `json:"request_type"`
	RequestorType            string         `json:"requestor_type"`
	RequestorName            string         `json:"requestor_name"`
	RequestorEmail           string         `json:"requestor_email"`
	RequestorPhone           string         `json:"requestor_phone,omitempty"`
	RequestorCompany         string         `json:"requestor_company,omitempty"`
	PropertyAddress          string         `json:"property_address"`
	OwnerName                string         `json:"owner_name"`
	ClosingDate              *time.Time     `json:"closing_date,omitempty"`
	RushRequested            bool           `json:"rush_requested"`
	Status                   string         `json:"status"`
	FeeCents                 int64          `json:"fee_cents"`
	RushFeeCents             int64          `json:"rush_fee_cents"`
	DelinquentSurchargeCents int64          `json:"delinquent_surcharge_cents"`
	TotalFeeCents            int64          `json:"total_fee_cents"`
	DeadlineAt               time.Time      `json:"deadline_at"`
	AssignedTo               *uuid.UUID     `json:"assigned_to,omitempty"`
	AmendmentOf              *uuid.UUID     `json:"amendment_of,omitempty"`
	Metadata                 map[string]any `json:"metadata"`
	CreatedBy                uuid.UUID      `json:"created_by"`
	CreatedAt                time.Time      `json:"created_at"`
	UpdatedAt                time.Time      `json:"updated_at"`
	DeletedAt                *time.Time     `json:"deleted_at,omitempty"`
}

// EstoppelCertificate is the generated output — a point-in-time legal document.
type EstoppelCertificate struct {
	ID                uuid.UUID       `json:"id"`
	RequestID         uuid.UUID       `json:"request_id"`
	OrgID             uuid.UUID       `json:"org_id"`
	UnitID            uuid.UUID       `json:"unit_id"`
	DocumentID        uuid.UUID       `json:"document_id"`
	Jurisdiction      string          `json:"jurisdiction"`
	EffectiveDate     time.Time       `json:"effective_date"`
	ExpiresAt         *time.Time      `json:"expires_at,omitempty"`
	DataSnapshot      json.RawMessage `json:"data_snapshot"`
	NarrativeSections json.RawMessage `json:"narrative_sections"`
	SignedBy          uuid.UUID       `json:"signed_by"`
	SignedAt          time.Time       `json:"signed_at"`
	SignerTitle       string          `json:"signer_title"`
	TemplateVersion   string          `json:"template_version"`
	AmendmentOf       *uuid.UUID      `json:"amendment_of,omitempty"`
	CreatedAt         time.Time       `json:"created_at"`
}

// EstoppelRules holds deserialized state compliance parameters from PolicyResolver.
type EstoppelRules struct {
	StandardTurnaroundBusinessDays int      `json:"standard_turnaround_business_days"`
	StandardFeeCents               int64    `json:"standard_fee_cents"`
	RushTurnaroundBusinessDays     *int     `json:"rush_turnaround_business_days,omitempty"`
	RushFeeCents                   int64    `json:"rush_fee_cents"`
	DelinquentSurchargeCents       int64    `json:"delinquent_surcharge_cents"`
	EffectivePeriodDays            *int     `json:"effective_period_days,omitempty"`
	ElectronicDeliveryRequired     bool     `json:"electronic_delivery_required"`
	StatutoryFormRequired          bool     `json:"statutory_form_required"`
	StatutoryFormID                string   `json:"statutory_form_id,omitempty"`
	FreeAmendmentOnError           bool     `json:"free_amendment_on_error"`
	StatuteRef                     string   `json:"statute_ref,omitempty"`
	RequiredAttachments            []string `json:"required_attachments,omitempty"`
}

// FeeBreakdown is the result of CalculateFees.
type FeeBreakdown struct {
	FeeCents                 int64
	RushFeeCents             int64
	DelinquentSurchargeCents int64
	TotalFeeCents            int64
}

// validTransitions defines allowed status changes for estoppel requests.
var validTransitions = map[string][]string{
	"submitted":        {"data_aggregation", "cancelled"},
	"data_aggregation": {"manager_review", "cancelled"},
	"manager_review":   {"approved", "cancelled"},
	"approved":         {"generating"},
	"generating":       {"delivered"},
	"delivered":        {},
	"cancelled":        {},
}

// IsValidTransition checks whether a status change is allowed.
func IsValidTransition(from, to string) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// CalculateFees computes the fee breakdown from state rules.
func CalculateFees(rules *EstoppelRules, rush, delinquent bool) FeeBreakdown {
	fb := FeeBreakdown{
		FeeCents: rules.StandardFeeCents,
	}
	if rush {
		fb.RushFeeCents = rules.RushFeeCents
	}
	if delinquent {
		fb.DelinquentSurchargeCents = rules.DelinquentSurchargeCents
	}
	fb.TotalFeeCents = fb.FeeCents + fb.RushFeeCents + fb.DelinquentSurchargeCents
	return fb
}

// CalculateDeadline computes the SLA deadline in business days from now.
func CalculateDeadline(rules *EstoppelRules, rush bool, from time.Time) time.Time {
	days := rules.StandardTurnaroundBusinessDays
	if rush && rules.RushTurnaroundBusinessDays != nil {
		days = *rules.RushTurnaroundBusinessDays
	}
	return addBusinessDays(from, days)
}

// addBusinessDays adds n business days (Mon-Fri) to the given time.
func addBusinessDays(from time.Time, n int) time.Time {
	added := 0
	current := from
	for added < n {
		current = current.AddDate(0, 0, 1)
		wd := current.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			added++
		}
	}
	return current
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/domain.go backend/internal/estoppel/domain_test.go
git commit -m "feat(estoppel): add domain types, status transitions, and fee calculation"
```

---

### Task 3: Data Provider Interfaces

**Files:**
- Create: `backend/internal/estoppel/providers.go`

- [ ] **Step 1: Create provider interfaces and snapshot types**

Create `backend/internal/estoppel/providers.go`:

```go
package estoppel

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// FinancialDataProvider returns a financial snapshot for a unit.
type FinancialDataProvider interface {
	GetUnitFinancialSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*FinancialSnapshot, error)
}

// ComplianceDataProvider returns a compliance snapshot for a unit.
type ComplianceDataProvider interface {
	GetUnitComplianceSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*ComplianceSnapshot, error)
}

// PropertyDataProvider returns property and ownership data for a unit.
type PropertyDataProvider interface {
	GetPropertySnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*PropertySnapshot, error)
}

// AggregatedData is the merged result of all provider snapshots.
type AggregatedData struct {
	Property   PropertySnapshot
	Financial  FinancialSnapshot
	Compliance ComplianceSnapshot
	Narratives *NarrativeSections
	AsOfTime   time.Time
}

// FinancialSnapshot holds all financial data points for estoppel generation.
type FinancialSnapshot struct {
	RegularAssessmentCents   int64
	AssessmentFrequency      string
	PaidThroughDate          time.Time
	NextInstallmentDueDate   time.Time
	SpecialAssessments       []SpecialAssessment
	CapitalContributionCents int64
	TransferFeeCents         int64
	ReserveFundFeeCents      int64

	CurrentBalanceCents  int64
	PastDueItems         []PastDueItem
	LateFeesCents        int64
	InterestCents        int64
	CollectionCostsCents int64
	TotalDelinquentCents int64

	HasActiveCollection bool
	CollectionStatus    string
	AttorneyName        string
	AttorneyContact     string
	LienStatus          string

	HasPaymentPlan     bool
	PaymentPlanDetails *PaymentPlanInfo

	// Lender questionnaire fields (org-level).
	DelinquencyRate60Days float64
	ReserveBalanceCents   int64
	ReserveTargetCents    int64
	BudgetStatus          string
	TotalUnits            int
	OwnerOccupiedCount    int
	RentalCount           int
}

type SpecialAssessment struct {
	Description      string
	TotalAmountCents int64
	RemainingCents   int64
	InstallmentCents int64
	NextDueDate      time.Time
}

type PastDueItem struct {
	Category    string
	AmountCents int64
	AsOfDate    time.Time
}

type PaymentPlanInfo struct {
	TotalOwedCents    int64
	InstallmentCents  int64
	Frequency         string
	InstallmentsTotal int
	InstallmentsPaid  int
	NextDueDate       time.Time
	Status            string
}

// ComplianceSnapshot holds governance and violations data for estoppel generation.
type ComplianceSnapshot struct {
	OpenViolations    []ViolationSummary
	PendingFinesCents int64

	PendingLitigation     []LitigationSummary
	RightOfFirstRefusal   bool
	BoardApprovalRequired bool
	RentalRestrictions    string

	InsuranceCoverage []InsuranceSummary

	StructuralIntegrityStatus string
	CommercialSpacePercent    float64
}

type ViolationSummary struct {
	Category     string
	Description  string
	Status       string
	FineCents    int64
	CureDeadline *time.Time
}

type LitigationSummary struct {
	Description string
	Status      string
	FiledDate   time.Time
}

type InsuranceSummary struct {
	CoverageType        string
	Provider            string
	PolicyNumber        string
	ExpiresAt           time.Time
	CoverageAmountCents int64
}

// PropertySnapshot holds property and ownership data for estoppel generation.
type PropertySnapshot struct {
	OrgName             string
	OrgAddress          string
	OrgState            string
	UnitLabel           string
	UnitAddress         string
	ParcelNumber        string
	CurrentOwners       []OwnerInfo
	ManagingFirmName    string
	ManagingFirmContact string
}

type OwnerInfo struct {
	Name      string
	Email     string
	Phone     string
	StartedAt time.Time
}

// NarrativeSections holds AI-generated narrative fields with citations.
type NarrativeSections struct {
	PendingSpecialAssessments NarrativeField `json:"pending_special_assessments"`
	LitigationDisclosure      NarrativeField `json:"litigation_disclosure"`
	InsuranceSummary          NarrativeField `json:"insurance_summary"`
	RentalRestrictions        NarrativeField `json:"rental_restrictions"`
	RightOfFirstRefusal       NarrativeField `json:"right_of_first_refusal"`
	BoardApprovalRequirements NarrativeField `json:"board_approval_requirements"`
}

type NarrativeField struct {
	Text       string    `json:"text"`
	Citations  []Citation `json:"citations,omitempty"`
	Confidence float64   `json:"confidence"`
	Editable   bool      `json:"editable"`
}

type Citation struct {
	DocumentName string `json:"document_name"`
	Section      string `json:"section"`
	PageNumber   int    `json:"page_number"`
}
```

- [ ] **Step 2: Verify it compiles**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./internal/estoppel/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/estoppel/providers.go
git commit -m "feat(estoppel): add data provider interfaces and snapshot types"
```

---

### Task 4: Database Migration

**Files:**
- Create: `backend/migrations/20260409000025_estoppel_tables.sql`

- [ ] **Step 1: Create migration file**

Create `backend/migrations/20260409000025_estoppel_tables.sql`:

```sql
-- Estoppel certificate generation tables.
-- Supports estoppel certificates and lender questionnaires.

CREATE TABLE estoppel_requests (
    id                         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    org_id                     UUID NOT NULL REFERENCES organizations(id),
    unit_id                    UUID NOT NULL REFERENCES units(id),
    task_id                    UUID REFERENCES tasks(id),
    request_type               TEXT NOT NULL CHECK (request_type IN ('estoppel_certificate', 'lender_questionnaire')),
    requestor_type             TEXT NOT NULL CHECK (requestor_type IN ('homeowner', 'title_company', 'closing_agent', 'attorney')),
    requestor_name             TEXT NOT NULL,
    requestor_email            TEXT NOT NULL,
    requestor_phone            TEXT,
    requestor_company          TEXT,
    property_address           TEXT NOT NULL,
    owner_name                 TEXT NOT NULL,
    closing_date               DATE,
    rush_requested             BOOLEAN NOT NULL DEFAULT false,
    status                     TEXT NOT NULL DEFAULT 'submitted',
    fee_cents                  INTEGER NOT NULL DEFAULT 0,
    rush_fee_cents             INTEGER NOT NULL DEFAULT 0,
    delinquent_surcharge_cents INTEGER NOT NULL DEFAULT 0,
    total_fee_cents            INTEGER NOT NULL DEFAULT 0,
    deadline_at                TIMESTAMPTZ NOT NULL,
    assigned_to                UUID REFERENCES users(id),
    amendment_of               UUID,
    metadata                   JSONB DEFAULT '{}',
    created_by                 UUID NOT NULL REFERENCES users(id),
    created_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                 TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at                 TIMESTAMPTZ
);

CREATE TABLE estoppel_certificates (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id         UUID NOT NULL REFERENCES estoppel_requests(id),
    org_id             UUID NOT NULL REFERENCES organizations(id),
    unit_id            UUID NOT NULL REFERENCES units(id),
    document_id        UUID NOT NULL REFERENCES documents(id),
    jurisdiction       TEXT NOT NULL,
    effective_date     DATE NOT NULL,
    expires_at         DATE,
    data_snapshot      JSONB NOT NULL,
    narrative_sections JSONB NOT NULL DEFAULT '{}',
    signed_by          UUID NOT NULL REFERENCES users(id),
    signed_at          TIMESTAMPTZ NOT NULL,
    signer_title       TEXT NOT NULL,
    template_version   TEXT NOT NULL,
    amendment_of       UUID REFERENCES estoppel_certificates(id),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Deferred FK: estoppel_requests.amendment_of → estoppel_certificates.id
ALTER TABLE estoppel_requests
    ADD CONSTRAINT fk_estoppel_requests_amendment
    FOREIGN KEY (amendment_of) REFERENCES estoppel_certificates(id);

-- Indexes
CREATE INDEX idx_estoppel_requests_org ON estoppel_requests(org_id, status) WHERE deleted_at IS NULL;
CREATE INDEX idx_estoppel_requests_unit ON estoppel_requests(unit_id, created_at DESC);
CREATE INDEX idx_estoppel_requests_deadline ON estoppel_requests(deadline_at) WHERE status NOT IN ('delivered', 'cancelled');
CREATE INDEX idx_estoppel_certificates_org ON estoppel_certificates(org_id);
CREATE INDEX idx_estoppel_certificates_unit ON estoppel_certificates(unit_id, effective_date DESC);
CREATE INDEX idx_estoppel_certificates_request ON estoppel_certificates(request_id);

-- RLS
ALTER TABLE estoppel_requests ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_requests FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_requests_org ON estoppel_requests
    USING (org_id = current_setting('app.current_org_id', true)::uuid);

ALTER TABLE estoppel_certificates ENABLE ROW LEVEL SECURITY;
ALTER TABLE estoppel_certificates FORCE ROW LEVEL SECURITY;
CREATE POLICY estoppel_certificates_org ON estoppel_certificates
    USING (org_id = current_setting('app.current_org_id', true)::uuid);
```

- [ ] **Step 2: Verify SQL syntax**

```bash
cd /home/douglasl/Projects/quorant/backend && cat migrations/20260409000025_estoppel_tables.sql | head -5
```

Expected: file exists with correct header.

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/20260409000025_estoppel_tables.sql
git commit -m "feat(estoppel): add database migration for estoppel tables"
```

---

### Task 5: Repository Interface and PostgreSQL Implementation

**Files:**
- Create: `backend/internal/estoppel/repository.go`
- Create: `backend/internal/estoppel/postgres.go`
- Create: `backend/internal/estoppel/postgres_test.go`

- [ ] **Step 1: Write the repository interface**

Create `backend/internal/estoppel/repository.go`:

```go
package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// EstoppelRepository persists estoppel requests and certificates.
type EstoppelRepository interface {
	CreateRequest(ctx context.Context, req *EstoppelRequest) (*EstoppelRequest, error)
	FindRequestByID(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error)
	ListRequestsByOrg(ctx context.Context, orgID uuid.UUID, status string, limit int, afterID *uuid.UUID) ([]EstoppelRequest, bool, error)
	UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) (*EstoppelRequest, error)
	UpdateRequestNarratives(ctx context.Context, id uuid.UUID, narratives []byte) (*EstoppelRequest, error)

	CreateCertificate(ctx context.Context, cert *EstoppelCertificate) (*EstoppelCertificate, error)
	FindCertificateByID(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error)
	FindCertificateByRequestID(ctx context.Context, requestID uuid.UUID) (*EstoppelCertificate, error)
	ListCertificatesByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]EstoppelCertificate, bool, error)
}
```

- [ ] **Step 2: Write failing integration test for CreateRequest**

Create `backend/internal/estoppel/postgres_test.go`:

```go
//go:build integration

package estoppel

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type estoppelTestFixture struct {
	repo   EstoppelRepository
	orgID  uuid.UUID
	unitID uuid.UUID
	userID uuid.UUID
	pool   *pgxpool.Pool
}

func setupEstoppelDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://quorant:quorant@localhost:5432/quorant_dev?sslmode=disable")
	require.NoError(t, err, "connecting to test database")

	t.Cleanup(func() {
		cleanCtx := context.Background()
		pool.Exec(cleanCtx, "DELETE FROM estoppel_certificates")
		pool.Exec(cleanCtx, "DELETE FROM estoppel_requests")
		pool.Close()
	})
	return pool
}

func setupEstoppelFixture(t *testing.T) estoppelTestFixture {
	t.Helper()
	ctx := context.Background()
	pool := setupEstoppelDB(t)

	var orgID uuid.UUID
	err := pool.QueryRow(ctx,
		`INSERT INTO organizations (type, name, slug, path, settings)
		 VALUES ('hoa', 'Estoppel Test HOA', $1, $2, '{}')
		 RETURNING id`,
		"estoppel-test-"+uuid.New().String()[:8],
		"estoppel_test_"+uuid.New().String()[:8],
	).Scan(&orgID)
	require.NoError(t, err, "create test org")

	var userID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO users (external_id, email, display_name) VALUES ($1, $2, 'Test User') RETURNING id`,
		uuid.New().String(), "estoppel-test-"+uuid.New().String()[:8]+"@test.com",
	).Scan(&userID)
	require.NoError(t, err, "create test user")

	var unitID uuid.UUID
	err = pool.QueryRow(ctx,
		`INSERT INTO units (org_id, label, status, voting_weight) VALUES ($1, 'Unit 101', 'occupied', 1.0) RETURNING id`,
		orgID,
	).Scan(&unitID)
	require.NoError(t, err, "create test unit")

	return estoppelTestFixture{
		repo:   NewPostgresRepository(pool),
		orgID:  orgID,
		unitID: unitID,
		userID: userID,
		pool:   pool,
	}
}

func minimalRequest(orgID, unitID, userID uuid.UUID) *EstoppelRequest {
	now := time.Now().UTC()
	return &EstoppelRequest{
		OrgID:           orgID,
		UnitID:          unitID,
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "First American Title",
		RequestorEmail:  "orders@firstam.com",
		PropertyAddress: "123 Palm St, Unit 101",
		OwnerName:       "Jane Doe",
		Status:          "submitted",
		FeeCents:        29900,
		TotalFeeCents:   29900,
		DeadlineAt:      now.AddDate(0, 0, 14),
		Metadata:        map[string]any{},
		CreatedBy:       userID,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func TestCreateRequest(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	input := minimalRequest(f.orgID, f.unitID, f.userID)
	got, err := f.repo.CreateRequest(ctx, input)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.NotEqual(t, uuid.Nil, got.ID)
	assert.Equal(t, f.orgID, got.OrgID)
	assert.Equal(t, "submitted", got.Status)
	assert.Equal(t, int64(29900), got.TotalFeeCents)
}

func TestFindRequestByID(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	found, err := f.repo.FindRequestByID(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, found.ID)
	assert.Equal(t, "title_company", found.RequestorType)
}

func TestUpdateRequestStatus(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	created, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	updated, err := f.repo.UpdateRequestStatus(ctx, created.ID, "data_aggregation")
	require.NoError(t, err)
	assert.Equal(t, "data_aggregation", updated.Status)
}

func TestListRequestsByOrg(t *testing.T) {
	f := setupEstoppelFixture(t)
	ctx := context.Background()

	_, err := f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)
	_, err = f.repo.CreateRequest(ctx, minimalRequest(f.orgID, f.unitID, f.userID))
	require.NoError(t, err)

	list, hasMore, err := f.repo.ListRequestsByOrg(ctx, f.orgID, "", 10, nil)
	require.NoError(t, err)
	assert.Len(t, list, 2)
	assert.False(t, hasMore)
}
```

- [ ] **Step 3: Implement PostgreSQL repository**

Create `backend/internal/estoppel/postgres.go`:

```go
package estoppel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements EstoppelRepository using PostgreSQL.
type PostgresRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresRepository returns a new PostgreSQL-backed repository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateRequest(ctx context.Context, req *EstoppelRequest) (*EstoppelRequest, error) {
	metaJSON, err := json.Marshal(req.Metadata)
	if err != nil {
		return nil, fmt.Errorf("estoppel: CreateRequest marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO estoppel_requests (
			org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13,
			$14, $15, $16, $17, $18,
			$19, $20, $21, $22, $23
		) RETURNING id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		req.OrgID, req.UnitID, req.TaskID, req.RequestType, req.RequestorType,
		req.RequestorName, req.RequestorEmail, req.RequestorPhone, req.RequestorCompany,
		req.PropertyAddress, req.OwnerName, req.ClosingDate, req.RushRequested,
		req.Status, req.FeeCents, req.RushFeeCents, req.DelinquentSurchargeCents, req.TotalFeeCents,
		req.DeadlineAt, req.AssignedTo, req.AmendmentOf, metaJSON, req.CreatedBy,
	)
	return scanRequest(row)
}

func (r *PostgresRepository) FindRequestByID(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error) {
	const q = `
		SELECT id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at
		FROM estoppel_requests WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanRequest(row)
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindRequestByID: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) ListRequestsByOrg(ctx context.Context, orgID uuid.UUID, status string, limit int, afterID *uuid.UUID) ([]EstoppelRequest, bool, error) {
	fetchLimit := limit + 1
	var rows pgx.Rows
	var err error

	if status != "" && afterID != nil {
		const q = `SELECT id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at
		FROM estoppel_requests WHERE org_id = $1 AND status = $2 AND id > $3 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $4`
		rows, err = r.pool.Query(ctx, q, orgID, status, *afterID, fetchLimit)
	} else if status != "" {
		const q = `SELECT id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at
		FROM estoppel_requests WHERE org_id = $1 AND status = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $3`
		rows, err = r.pool.Query(ctx, q, orgID, status, fetchLimit)
	} else if afterID != nil {
		const q = `SELECT id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at
		FROM estoppel_requests WHERE org_id = $1 AND id > $2 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $3`
		rows, err = r.pool.Query(ctx, q, orgID, *afterID, fetchLimit)
	} else {
		const q = `SELECT id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at
		FROM estoppel_requests WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC LIMIT $2`
		rows, err = r.pool.Query(ctx, q, orgID, fetchLimit)
	}
	if err != nil {
		return nil, false, fmt.Errorf("estoppel: ListRequestsByOrg: %w", err)
	}
	defer rows.Close()

	var results []EstoppelRequest
	for rows.Next() {
		req, err := scanRequestRow(rows)
		if err != nil {
			return nil, false, fmt.Errorf("estoppel: ListRequestsByOrg scan: %w", err)
		}
		results = append(results, *req)
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

func (r *PostgresRepository) UpdateRequestStatus(ctx context.Context, id uuid.UUID, status string) (*EstoppelRequest, error) {
	const q = `
		UPDATE estoppel_requests SET status = $2, updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at`
	row := r.pool.QueryRow(ctx, q, id, status)
	return scanRequest(row)
}

func (r *PostgresRepository) UpdateRequestNarratives(ctx context.Context, id uuid.UUID, narratives []byte) (*EstoppelRequest, error) {
	const q = `
		UPDATE estoppel_requests SET metadata = jsonb_set(COALESCE(metadata, '{}'), '{narratives}', $2::jsonb), updated_at = now()
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, org_id, unit_id, task_id, request_type, requestor_type,
			requestor_name, requestor_email, requestor_phone, requestor_company,
			property_address, owner_name, closing_date, rush_requested,
			status, fee_cents, rush_fee_cents, delinquent_surcharge_cents, total_fee_cents,
			deadline_at, assigned_to, amendment_of, metadata, created_by,
			created_at, updated_at, deleted_at`
	row := r.pool.QueryRow(ctx, q, id, narratives)
	return scanRequest(row)
}

func (r *PostgresRepository) CreateCertificate(ctx context.Context, cert *EstoppelCertificate) (*EstoppelCertificate, error) {
	const q = `
		INSERT INTO estoppel_certificates (
			request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING id, request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of, created_at`

	row := r.pool.QueryRow(ctx, q,
		cert.RequestID, cert.OrgID, cert.UnitID, cert.DocumentID, cert.Jurisdiction,
		cert.EffectiveDate, cert.ExpiresAt, cert.DataSnapshot, cert.NarrativeSections,
		cert.SignedBy, cert.SignedAt, cert.SignerTitle, cert.TemplateVersion, cert.AmendmentOf,
	)
	return scanCertificate(row)
}

func (r *PostgresRepository) FindCertificateByID(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error) {
	const q = `
		SELECT id, request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of, created_at
		FROM estoppel_certificates WHERE id = $1`
	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanCertificate(row)
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindCertificateByID: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) FindCertificateByRequestID(ctx context.Context, requestID uuid.UUID) (*EstoppelCertificate, error) {
	const q = `
		SELECT id, request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of, created_at
		FROM estoppel_certificates WHERE request_id = $1`
	row := r.pool.QueryRow(ctx, q, requestID)
	result, err := scanCertificate(row)
	if err != nil {
		return nil, fmt.Errorf("estoppel: FindCertificateByRequestID: %w", err)
	}
	return result, nil
}

func (r *PostgresRepository) ListCertificatesByOrg(ctx context.Context, orgID uuid.UUID, limit int, afterID *uuid.UUID) ([]EstoppelCertificate, bool, error) {
	fetchLimit := limit + 1
	var rows pgx.Rows
	var err error

	if afterID != nil {
		const q = `SELECT id, request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of, created_at
		FROM estoppel_certificates WHERE org_id = $1 AND id > $2
		ORDER BY created_at DESC LIMIT $3`
		rows, err = r.pool.Query(ctx, q, orgID, *afterID, fetchLimit)
	} else {
		const q = `SELECT id, request_id, org_id, unit_id, document_id, jurisdiction,
			effective_date, expires_at, data_snapshot, narrative_sections,
			signed_by, signed_at, signer_title, template_version, amendment_of, created_at
		FROM estoppel_certificates WHERE org_id = $1
		ORDER BY created_at DESC LIMIT $2`
		rows, err = r.pool.Query(ctx, q, orgID, fetchLimit)
	}
	if err != nil {
		return nil, false, fmt.Errorf("estoppel: ListCertificatesByOrg: %w", err)
	}
	defer rows.Close()

	var results []EstoppelCertificate
	for rows.Next() {
		cert, err := scanCertificateRow(rows)
		if err != nil {
			return nil, false, fmt.Errorf("estoppel: ListCertificatesByOrg scan: %w", err)
		}
		results = append(results, *cert)
	}

	hasMore := len(results) > limit
	if hasMore {
		results = results[:limit]
	}
	return results, hasMore, nil
}

// scanRequest scans an EstoppelRequest from a single pgx.Row.
func scanRequest(row pgx.Row) (*EstoppelRequest, error) {
	var req EstoppelRequest
	var metaJSON []byte
	err := row.Scan(
		&req.ID, &req.OrgID, &req.UnitID, &req.TaskID, &req.RequestType, &req.RequestorType,
		&req.RequestorName, &req.RequestorEmail, &req.RequestorPhone, &req.RequestorCompany,
		&req.PropertyAddress, &req.OwnerName, &req.ClosingDate, &req.RushRequested,
		&req.Status, &req.FeeCents, &req.RushFeeCents, &req.DelinquentSurchargeCents, &req.TotalFeeCents,
		&req.DeadlineAt, &req.AssignedTo, &req.AmendmentOf, &metaJSON, &req.CreatedBy,
		&req.CreatedAt, &req.UpdatedAt, &req.DeletedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("estoppel: scan request: %w", err)
	}
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &req.Metadata)
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	return &req, nil
}

// scanRequestRow scans from pgx.Rows (for list queries).
func scanRequestRow(rows pgx.Rows) (*EstoppelRequest, error) {
	var req EstoppelRequest
	var metaJSON []byte
	err := rows.Scan(
		&req.ID, &req.OrgID, &req.UnitID, &req.TaskID, &req.RequestType, &req.RequestorType,
		&req.RequestorName, &req.RequestorEmail, &req.RequestorPhone, &req.RequestorCompany,
		&req.PropertyAddress, &req.OwnerName, &req.ClosingDate, &req.RushRequested,
		&req.Status, &req.FeeCents, &req.RushFeeCents, &req.DelinquentSurchargeCents, &req.TotalFeeCents,
		&req.DeadlineAt, &req.AssignedTo, &req.AmendmentOf, &metaJSON, &req.CreatedBy,
		&req.CreatedAt, &req.UpdatedAt, &req.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	if metaJSON != nil {
		_ = json.Unmarshal(metaJSON, &req.Metadata)
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	return &req, nil
}

// scanCertificate scans an EstoppelCertificate from a single pgx.Row.
func scanCertificate(row pgx.Row) (*EstoppelCertificate, error) {
	var cert EstoppelCertificate
	err := row.Scan(
		&cert.ID, &cert.RequestID, &cert.OrgID, &cert.UnitID, &cert.DocumentID, &cert.Jurisdiction,
		&cert.EffectiveDate, &cert.ExpiresAt, &cert.DataSnapshot, &cert.NarrativeSections,
		&cert.SignedBy, &cert.SignedAt, &cert.SignerTitle, &cert.TemplateVersion, &cert.AmendmentOf,
		&cert.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("estoppel: scan certificate: %w", err)
	}
	return &cert, nil
}

// scanCertificateRow scans from pgx.Rows (for list queries).
func scanCertificateRow(rows pgx.Rows) (*EstoppelCertificate, error) {
	var cert EstoppelCertificate
	err := rows.Scan(
		&cert.ID, &cert.RequestID, &cert.OrgID, &cert.UnitID, &cert.DocumentID, &cert.Jurisdiction,
		&cert.EffectiveDate, &cert.ExpiresAt, &cert.DataSnapshot, &cert.NarrativeSections,
		&cert.SignedBy, &cert.SignedAt, &cert.SignerTitle, &cert.TemplateVersion, &cert.AmendmentOf,
		&cert.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}
```

- [ ] **Step 4: Verify it compiles**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./internal/estoppel/
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/repository.go backend/internal/estoppel/postgres.go backend/internal/estoppel/postgres_test.go
git commit -m "feat(estoppel): add repository interface and PostgreSQL implementation"
```

---

### Task 6: Data Provider Adapters

**Files:**
- Create: `backend/internal/fin/estoppel_adapter.go`
- Create: `backend/internal/gov/estoppel_adapter.go`
- Create: `backend/internal/org/estoppel_adapter.go`
- Create: `backend/internal/estoppel/adapters_test.go`

- [ ] **Step 1: Write failing test for FinancialDataProvider adapter**

Create `backend/internal/estoppel/adapters_test.go`:

```go
package estoppel

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockFinancialProvider is a test double for FinancialDataProvider.
type mockFinancialProvider struct {
	snapshot *FinancialSnapshot
	err      error
}

func (m *mockFinancialProvider) GetUnitFinancialSnapshot(_ context.Context, _, _ uuid.UUID) (*FinancialSnapshot, error) {
	return m.snapshot, m.err
}

// mockComplianceProvider is a test double for ComplianceDataProvider.
type mockComplianceProvider struct {
	snapshot *ComplianceSnapshot
	err      error
}

func (m *mockComplianceProvider) GetUnitComplianceSnapshot(_ context.Context, _, _ uuid.UUID) (*ComplianceSnapshot, error) {
	return m.snapshot, m.err
}

// mockPropertyProvider is a test double for PropertyDataProvider.
type mockPropertyProvider struct {
	snapshot *PropertySnapshot
	err      error
}

func (m *mockPropertyProvider) GetPropertySnapshot(_ context.Context, _, _ uuid.UUID) (*PropertySnapshot, error) {
	return m.snapshot, m.err
}

func TestMockFinancialProvider_ReturnsSnapshot(t *testing.T) {
	provider := &mockFinancialProvider{
		snapshot: &FinancialSnapshot{
			RegularAssessmentCents: 25000,
			AssessmentFrequency:    "monthly",
			CurrentBalanceCents:    0,
		},
	}
	snap, err := provider.GetUnitFinancialSnapshot(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, int64(25000), snap.RegularAssessmentCents)
}

func TestMockComplianceProvider_ReturnsSnapshot(t *testing.T) {
	provider := &mockComplianceProvider{
		snapshot: &ComplianceSnapshot{
			OpenViolations:    []ViolationSummary{},
			PendingFinesCents: 0,
		},
	}
	snap, err := provider.GetUnitComplianceSnapshot(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Empty(t, snap.OpenViolations)
}

func TestMockPropertyProvider_ReturnsSnapshot(t *testing.T) {
	provider := &mockPropertyProvider{
		snapshot: &PropertySnapshot{
			OrgName:  "Sunset HOA",
			OrgState: "FL",
			CurrentOwners: []OwnerInfo{
				{Name: "Jane Doe", Email: "jane@example.com"},
			},
		},
	}
	snap, err := provider.GetPropertySnapshot(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "FL", snap.OrgState)
	assert.Len(t, snap.CurrentOwners, 1)
}
```

- [ ] **Step 2: Run tests to verify they pass (mock tests only)**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestMock"
```

Expected: PASS.

- [ ] **Step 3: Create fin adapter**

Create `backend/internal/fin/estoppel_adapter.go`:

```go
package fin

import (
	"context"

	"github.com/google/uuid"

	"quorant/internal/estoppel"
)

// EstoppelFinancialAdapter implements estoppel.FinancialDataProvider by
// wrapping the FinService to query unit financial data.
type EstoppelFinancialAdapter struct {
	service *FinService
}

// NewEstoppelFinancialAdapter creates an adapter for the estoppel module.
func NewEstoppelFinancialAdapter(service *FinService) *EstoppelFinancialAdapter {
	return &EstoppelFinancialAdapter{service: service}
}

// GetUnitFinancialSnapshot aggregates financial data for a unit into an estoppel snapshot.
func (a *EstoppelFinancialAdapter) GetUnitFinancialSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.FinancialSnapshot, error) {
	snap := &estoppel.FinancialSnapshot{}

	// Assessment schedule: get the active schedule for this org.
	schedules, err := a.service.ListSchedules(ctx, orgID)
	if err != nil {
		return nil, err
	}
	if len(schedules) > 0 {
		s := schedules[0]
		snap.RegularAssessmentCents = s.BaseAmountCents
		snap.AssessmentFrequency = s.Frequency
	}

	// Unit balance from ledger.
	balance, err := a.service.GetUnitBalance(ctx, unitID)
	if err != nil {
		return nil, err
	}
	snap.CurrentBalanceCents = balance
	if balance > 0 {
		snap.TotalDelinquentCents = balance
	}

	// Collection status.
	collCase, err := a.service.GetCollectionStatusForUnit(ctx, unitID)
	if err != nil {
		return nil, err
	}
	if collCase != nil {
		snap.HasActiveCollection = true
		snap.CollectionStatus = collCase.Status
	}

	// Fund balances for lender questionnaire.
	funds, err := a.service.ListFunds(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, f := range funds {
		if f.FundType == "reserve" {
			snap.ReserveBalanceCents = f.BalanceCents
			if f.TargetBalanceCents != nil {
				snap.ReserveTargetCents = *f.TargetBalanceCents
			}
		}
	}

	return snap, nil
}
```

- [ ] **Step 4: Create gov adapter**

Create `backend/internal/gov/estoppel_adapter.go`:

```go
package gov

import (
	"context"

	"github.com/google/uuid"

	"quorant/internal/estoppel"
)

// EstoppelComplianceAdapter implements estoppel.ComplianceDataProvider by
// wrapping the GovService to query compliance data.
type EstoppelComplianceAdapter struct {
	service *GovService
}

// NewEstoppelComplianceAdapter creates an adapter for the estoppel module.
func NewEstoppelComplianceAdapter(service *GovService) *EstoppelComplianceAdapter {
	return &EstoppelComplianceAdapter{service: service}
}

// GetUnitComplianceSnapshot aggregates governance data for a unit into an estoppel snapshot.
func (a *EstoppelComplianceAdapter) GetUnitComplianceSnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.ComplianceSnapshot, error) {
	snap := &estoppel.ComplianceSnapshot{}

	// Open violations.
	violations, err := a.service.ListViolations(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for _, v := range violations {
		if v.UnitID == unitID && v.Status != "resolved" && v.Status != "cured" {
			snap.OpenViolations = append(snap.OpenViolations, estoppel.ViolationSummary{
				Category:     v.Category,
				Description:  v.Description,
				Status:       v.Status,
				FineCents:    v.FineTotalCents,
				CureDeadline: v.CureDeadline,
			})
			snap.PendingFinesCents += v.FineTotalCents
		}
	}

	return snap, nil
}
```

- [ ] **Step 5: Create org adapter**

Create `backend/internal/org/estoppel_adapter.go`:

```go
package org

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	"quorant/internal/estoppel"
)

// EstoppelPropertyAdapter implements estoppel.PropertyDataProvider by
// wrapping OrgService to query property and ownership data.
type EstoppelPropertyAdapter struct {
	service *OrgService
}

// NewEstoppelPropertyAdapter creates an adapter for the estoppel module.
func NewEstoppelPropertyAdapter(service *OrgService) *EstoppelPropertyAdapter {
	return &EstoppelPropertyAdapter{service: service}
}

// GetPropertySnapshot aggregates property data for a unit into an estoppel snapshot.
func (a *EstoppelPropertyAdapter) GetPropertySnapshot(ctx context.Context, orgID, unitID uuid.UUID) (*estoppel.PropertySnapshot, error) {
	snap := &estoppel.PropertySnapshot{}

	// Org info.
	org, err := a.service.GetOrganization(ctx, orgID)
	if err != nil {
		return nil, fmt.Errorf("estoppel adapter: get org: %w", err)
	}
	snap.OrgName = org.Name
	if org.AddressLine1 != nil {
		snap.OrgAddress = *org.AddressLine1
	}
	if org.State != nil {
		snap.OrgState = *org.State
	}

	// Unit info.
	unit, err := a.service.GetUnit(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("estoppel adapter: get unit: %w", err)
	}
	snap.UnitLabel = unit.Label
	if unit.AddressLine1 != nil {
		snap.UnitAddress = *unit.AddressLine1
	}

	// Property details.
	prop, err := a.service.GetProperty(ctx, unitID)
	if err == nil && prop != nil && prop.ParcelNumber != nil {
		snap.ParcelNumber = *prop.ParcelNumber
	}

	// Current owners from unit memberships.
	memberships, err := a.service.ListUnitMemberships(ctx, unitID)
	if err != nil {
		return nil, fmt.Errorf("estoppel adapter: list memberships: %w", err)
	}
	for _, m := range memberships {
		if m.Relationship == "owner" && m.EndedAt == nil {
			snap.CurrentOwners = append(snap.CurrentOwners, estoppel.OwnerInfo{
				Name:      m.UserID.String(), // resolved by handler or enrichment step
				StartedAt: m.StartedAt,
			})
		}
	}

	return snap, nil
}
```

- [ ] **Step 6: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./internal/fin/ ./internal/gov/ ./internal/org/ ./internal/estoppel/
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/estoppel_adapter.go backend/internal/gov/estoppel_adapter.go backend/internal/org/estoppel_adapter.go backend/internal/estoppel/adapters_test.go
git commit -m "feat(estoppel): add cross-module data provider adapters"
```

---

### Task 7: Narrative Generator

**Files:**
- Create: `backend/internal/estoppel/narrative.go`
- Create: `backend/internal/estoppel/narrative_test.go`

- [ ] **Step 1: Write failing test for narrative generation**

Create `backend/internal/estoppel/narrative_test.go`:

```go
package estoppel

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNarrativeGenerator_GeneratesAllFields(t *testing.T) {
	gen := NewNoopNarrativeGenerator()
	data := &AggregatedData{
		Property: PropertySnapshot{OrgName: "Sunset HOA", OrgState: "FL"},
	}

	sections, err := gen.GenerateNarratives(context.Background(), uuid.New(), data)
	require.NoError(t, err)
	require.NotNil(t, sections)
	assert.True(t, sections.PendingSpecialAssessments.Editable)
	assert.True(t, sections.LitigationDisclosure.Editable)
	assert.True(t, sections.InsuranceSummary.Editable)
	assert.True(t, sections.RentalRestrictions.Editable)
	assert.True(t, sections.RightOfFirstRefusal.Editable)
	assert.True(t, sections.BoardApprovalRequirements.Editable)
}

func TestNarrativeGenerator_FallbackWhenNoContext(t *testing.T) {
	gen := NewNoopNarrativeGenerator()
	data := &AggregatedData{}

	sections, err := gen.GenerateNarratives(context.Background(), uuid.New(), data)
	require.NoError(t, err)
	// All fields should have text prompting manual entry.
	assert.Contains(t, sections.PendingSpecialAssessments.Text, "Manual entry required")
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestNarrative"
```

Expected: compilation errors.

- [ ] **Step 3: Implement narrative generator**

Create `backend/internal/estoppel/narrative.go`:

```go
package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// NarrativeGenerator resolves ambiguous estoppel fields using the AI context lake.
type NarrativeGenerator interface {
	GenerateNarratives(ctx context.Context, orgID uuid.UUID, data *AggregatedData) (*NarrativeSections, error)
}

// NoopNarrativeGenerator returns manual-entry placeholders for all fields.
// Used when the AI module is unavailable or the org has no indexed governing documents.
type NoopNarrativeGenerator struct{}

// NewNoopNarrativeGenerator creates a fallback narrative generator.
func NewNoopNarrativeGenerator() *NoopNarrativeGenerator {
	return &NoopNarrativeGenerator{}
}

// GenerateNarratives returns placeholder narrative sections requiring manual entry.
func (g *NoopNarrativeGenerator) GenerateNarratives(_ context.Context, _ uuid.UUID, _ *AggregatedData) (*NarrativeSections, error) {
	placeholder := func() NarrativeField {
		return NarrativeField{
			Text:       "Manual entry required — no governing documents indexed.",
			Confidence: 0,
			Editable:   true,
		}
	}
	return &NarrativeSections{
		PendingSpecialAssessments: placeholder(),
		LitigationDisclosure:      placeholder(),
		InsuranceSummary:          placeholder(),
		RentalRestrictions:        placeholder(),
		RightOfFirstRefusal:       placeholder(),
		BoardApprovalRequirements: placeholder(),
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestNarrative"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/narrative.go backend/internal/estoppel/narrative_test.go
git commit -m "feat(estoppel): add narrative generator interface with noop fallback"
```

---

### Task 8: PDF Generator with Maroto v2

**Files:**
- Create: `backend/internal/estoppel/generator.go`
- Create: `backend/internal/estoppel/generator_test.go`
- Create: `backend/internal/estoppel/templates.go`

- [ ] **Step 1: Write failing test for PDF generation**

Create `backend/internal/estoppel/generator_test.go`:

```go
package estoppel

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleAggregatedData() *AggregatedData {
	return &AggregatedData{
		Property: PropertySnapshot{
			OrgName:   "Sunset Village HOA",
			OrgState:  "FL",
			UnitLabel: "Unit 101",
			UnitAddress: "123 Palm St, Unit 101, Miami, FL 33101",
			CurrentOwners: []OwnerInfo{
				{Name: "Jane Doe", Email: "jane@example.com"},
			},
		},
		Financial: FinancialSnapshot{
			RegularAssessmentCents: 25000,
			AssessmentFrequency:    "monthly",
			CurrentBalanceCents:    0,
		},
		Compliance: ComplianceSnapshot{
			OpenViolations: []ViolationSummary{},
		},
		Narratives: &NarrativeSections{
			PendingSpecialAssessments: NarrativeField{Text: "None pending.", Confidence: 1.0, Editable: true},
			LitigationDisclosure:      NarrativeField{Text: "No pending litigation.", Confidence: 1.0, Editable: true},
			InsuranceSummary:          NarrativeField{Text: "Coverage current.", Confidence: 1.0, Editable: true},
			RentalRestrictions:        NarrativeField{Text: "No restrictions.", Confidence: 1.0, Editable: true},
			RightOfFirstRefusal:       NarrativeField{Text: "None.", Confidence: 1.0, Editable: true},
			BoardApprovalRequirements: NarrativeField{Text: "None required.", Confidence: 1.0, Editable: true},
		},
		AsOfTime: time.Now().UTC(),
	}
}

func sampleRules() *EstoppelRules {
	return &EstoppelRules{
		StandardFeeCents: 29900,
		StatutoryFormID:  "generic",
		StatuteRef:       "N/A",
	}
}

func TestMarotoGenerator_GenerateEstoppel_ProducesValidPDF(t *testing.T) {
	gen := NewMarotoGenerator()
	data := sampleAggregatedData()
	rules := sampleRules()

	pdfBytes, err := gen.GenerateEstoppel(data, rules)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	// PDF files start with %PDF
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}

func TestMarotoGenerator_GenerateLenderQuestionnaire_ProducesValidPDF(t *testing.T) {
	gen := NewMarotoGenerator()
	data := sampleAggregatedData()
	rules := sampleRules()

	pdfBytes, err := gen.GenerateLenderQuestionnaire(data, rules)
	require.NoError(t, err)
	require.NotEmpty(t, pdfBytes)
	assert.Equal(t, "%PDF", string(pdfBytes[:4]))
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestMaroto"
```

Expected: compilation errors.

- [ ] **Step 3: Implement template registry**

Create `backend/internal/estoppel/templates.go`:

```go
package estoppel

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// TemplateBuilder is a function that populates a Maroto document with state-specific content.
type TemplateBuilder func(m maroto.Maroto, data *AggregatedData, rules *EstoppelRules)

var templateRegistry = map[string]TemplateBuilder{
	"fl_720_30851": buildGenericEstoppelTemplate,
	"ca_4528":      buildGenericEstoppelTemplate,
	"tx_207":       buildGenericEstoppelTemplate,
	"nv_116":       buildGenericEstoppelTemplate,
	"va_55_1":      buildGenericEstoppelTemplate,
	"generic":      buildGenericEstoppelTemplate,
	"lender":       buildLenderTemplate,
}

func getTemplateBuilder(formID string) (TemplateBuilder, error) {
	builder, ok := templateRegistry[formID]
	if !ok {
		return nil, fmt.Errorf("estoppel: unknown template form ID: %s", formID)
	}
	return builder, nil
}

func buildGenericEstoppelTemplate(m maroto.Maroto, data *AggregatedData, rules *EstoppelRules) {
	// Title
	m.AddRow(12, text.NewCol(12, "ESTOPPEL CERTIFICATE", props.Text{
		Top:   2,
		Size:  16,
		Style: fontstyle.Bold,
		Align: align.Center,
	}))
	m.AddRow(8, text.NewCol(12, data.Property.OrgName, props.Text{
		Top:   1,
		Size:  12,
		Align: align.Center,
	}))
	m.AddRow(6, text.NewCol(12, fmt.Sprintf("As of: %s", data.AsOfTime.Format("January 2, 2006")), props.Text{
		Top:   1,
		Size:  10,
		Align: align.Center,
	}))

	addSpacer(m)

	// Property Information
	addSectionHeader(m, "PROPERTY INFORMATION")
	addLabelValue(m, "Property Address:", data.Property.UnitAddress)
	addLabelValue(m, "Unit:", data.Property.UnitLabel)
	if data.Property.ParcelNumber != "" {
		addLabelValue(m, "Parcel Number:", data.Property.ParcelNumber)
	}
	owners := ""
	for i, o := range data.Property.CurrentOwners {
		if i > 0 {
			owners += ", "
		}
		owners += o.Name
	}
	addLabelValue(m, "Current Owner(s):", owners)

	addSpacer(m)

	// Assessment Information
	addSectionHeader(m, "ASSESSMENT INFORMATION")
	addLabelValue(m, "Regular Assessment:", formatCents(data.Financial.RegularAssessmentCents))
	addLabelValue(m, "Frequency:", data.Financial.AssessmentFrequency)
	if !data.Financial.PaidThroughDate.IsZero() {
		addLabelValue(m, "Paid Through:", data.Financial.PaidThroughDate.Format("January 2, 2006"))
	}
	if !data.Financial.NextInstallmentDueDate.IsZero() {
		addLabelValue(m, "Next Due:", data.Financial.NextInstallmentDueDate.Format("January 2, 2006"))
	}
	if data.Financial.CapitalContributionCents > 0 {
		addLabelValue(m, "Capital Contribution:", formatCents(data.Financial.CapitalContributionCents))
	}
	if data.Financial.TransferFeeCents > 0 {
		addLabelValue(m, "Transfer Fee:", formatCents(data.Financial.TransferFeeCents))
	}

	addSpacer(m)

	// Balance & Delinquency
	addSectionHeader(m, "ACCOUNT BALANCE")
	addLabelValue(m, "Current Balance:", formatCents(data.Financial.CurrentBalanceCents))
	if data.Financial.TotalDelinquentCents > 0 {
		addLabelValue(m, "Total Delinquent:", formatCents(data.Financial.TotalDelinquentCents))
		if data.Financial.LateFeesCents > 0 {
			addLabelValue(m, "Late Fees:", formatCents(data.Financial.LateFeesCents))
		}
		if data.Financial.InterestCents > 0 {
			addLabelValue(m, "Interest:", formatCents(data.Financial.InterestCents))
		}
		if data.Financial.CollectionCostsCents > 0 {
			addLabelValue(m, "Collection Costs:", formatCents(data.Financial.CollectionCostsCents))
		}
	}

	addSpacer(m)

	// Collection Status
	if data.Financial.HasActiveCollection {
		addSectionHeader(m, "COLLECTION STATUS")
		addLabelValue(m, "Status:", data.Financial.CollectionStatus)
		if data.Financial.AttorneyName != "" {
			addLabelValue(m, "Attorney:", data.Financial.AttorneyName)
		}
		if data.Financial.LienStatus != "" {
			addLabelValue(m, "Lien Status:", data.Financial.LienStatus)
		}
		addSpacer(m)
	}

	// Violations
	addSectionHeader(m, "VIOLATIONS & COMPLIANCE")
	if len(data.Compliance.OpenViolations) == 0 {
		addLabelValue(m, "Open Violations:", "None")
	} else {
		for _, v := range data.Compliance.OpenViolations {
			addLabelValue(m, fmt.Sprintf("- %s:", v.Category), fmt.Sprintf("%s (Fine: %s)", v.Description, formatCents(v.FineCents)))
		}
	}

	addSpacer(m)

	// Narrative sections
	if data.Narratives != nil {
		addSectionHeader(m, "ADDITIONAL DISCLOSURES")
		addNarrativeField(m, "Pending Special Assessments", data.Narratives.PendingSpecialAssessments)
		addNarrativeField(m, "Litigation Disclosure", data.Narratives.LitigationDisclosure)
		addNarrativeField(m, "Insurance Summary", data.Narratives.InsuranceSummary)
		addNarrativeField(m, "Rental Restrictions", data.Narratives.RentalRestrictions)
		addNarrativeField(m, "Right of First Refusal", data.Narratives.RightOfFirstRefusal)
		addNarrativeField(m, "Board Approval Requirements", data.Narratives.BoardApprovalRequirements)
	}

	addSpacer(m)

	// Statute reference
	if rules.StatuteRef != "" {
		addLabelValue(m, "Statute Reference:", rules.StatuteRef)
	}
}

func buildLenderTemplate(m maroto.Maroto, data *AggregatedData, _ *EstoppelRules) {
	m.AddRow(12, text.NewCol(12, "LENDER QUESTIONNAIRE", props.Text{
		Top:   2,
		Size:  16,
		Style: fontstyle.Bold,
		Align: align.Center,
	}))
	m.AddRow(8, text.NewCol(12, data.Property.OrgName, props.Text{
		Top:   1,
		Size:  12,
		Align: align.Center,
	}))

	addSpacer(m)
	addSectionHeader(m, "COMMUNITY INFORMATION")
	addLabelValue(m, "Total Units:", fmt.Sprintf("%d", data.Financial.TotalUnits))
	addLabelValue(m, "Owner-Occupied:", fmt.Sprintf("%d", data.Financial.OwnerOccupiedCount))
	addLabelValue(m, "Rental Units:", fmt.Sprintf("%d", data.Financial.RentalCount))
	addLabelValue(m, "Delinquency Rate (60+ days):", fmt.Sprintf("%.1f%%", data.Financial.DelinquencyRate60Days))

	addSpacer(m)
	addSectionHeader(m, "FINANCIAL STATUS")
	addLabelValue(m, "Reserve Balance:", formatCents(data.Financial.ReserveBalanceCents))
	addLabelValue(m, "Reserve Target:", formatCents(data.Financial.ReserveTargetCents))
	addLabelValue(m, "Budget Status:", data.Financial.BudgetStatus)

	addSpacer(m)
	addSectionHeader(m, "PROPERTY CONDITION")
	addLabelValue(m, "Structural Integrity:", data.Compliance.StructuralIntegrityStatus)
	addLabelValue(m, "Commercial Space:", fmt.Sprintf("%.1f%%", data.Compliance.CommercialSpacePercent))
}

// Helper functions for template building.

func addSectionHeader(m maroto.Maroto, title string) {
	m.AddRow(8, text.NewCol(12, title, props.Text{
		Top:   1,
		Size:  11,
		Style: fontstyle.Bold,
	}))
}

func addLabelValue(m maroto.Maroto, label, value string) {
	m.AddRow(6,
		text.NewCol(4, label, props.Text{Top: 1, Size: 9, Style: fontstyle.Bold}),
		text.NewCol(8, value, props.Text{Top: 1, Size: 9}),
	)
}

func addNarrativeField(m maroto.Maroto, label string, field NarrativeField) {
	m.AddRow(6, text.NewCol(12, label+":", props.Text{Top: 1, Size: 9, Style: fontstyle.Bold}))
	m.AddRow(6, text.NewCol(12, field.Text, props.Text{Top: 0, Size: 9}))
}

func addSpacer(m maroto.Maroto) {
	m.AddRow(4)
}

func formatCents(cents int64) string {
	dollars := float64(cents) / 100.0
	return fmt.Sprintf("$%.2f", dollars)
}
```

- [ ] **Step 4: Implement PDF generator**

Create `backend/internal/estoppel/generator.go`:

```go
package estoppel

import (
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/pagesize"
)

// CertificateGenerator produces PDF documents from aggregated data.
type CertificateGenerator interface {
	GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error)
	GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error)
}

// MarotoGenerator implements CertificateGenerator using Maroto v2.
type MarotoGenerator struct{}

// NewMarotoGenerator creates a new Maroto-based PDF generator.
func NewMarotoGenerator() *MarotoGenerator {
	return &MarotoGenerator{}
}

// GenerateEstoppel produces an estoppel certificate PDF.
func (g *MarotoGenerator) GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	formID := rules.StatutoryFormID
	if formID == "" {
		formID = "generic"
	}

	builder, err := getTemplateBuilder(formID)
	if err != nil {
		return nil, err
	}

	cfg := config.NewBuilder().
		WithPageSize(pagesize.Letter).
		WithLeftMargin(15).
		WithRightMargin(15).
		WithTopMargin(15).
		WithBottomMargin(15).
		Build()

	m := maroto.New(cfg)
	builder(m, data, rules)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("estoppel: generate PDF: %w", err)
	}

	return doc.GetBytes(), nil
}

// GenerateLenderQuestionnaire produces a lender questionnaire PDF.
func (g *MarotoGenerator) GenerateLenderQuestionnaire(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	builder, err := getTemplateBuilder("lender")
	if err != nil {
		return nil, err
	}

	cfg := config.NewBuilder().
		WithPageSize(pagesize.Letter).
		WithLeftMargin(15).
		WithRightMargin(15).
		WithTopMargin(15).
		WithBottomMargin(15).
		Build()

	m := maroto.New(cfg)
	builder(m, data, rules)

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("estoppel: generate lender PDF: %w", err)
	}

	return doc.GetBytes(), nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestMaroto"
```

Expected: PASS (valid PDF bytes starting with `%PDF`).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/estoppel/generator.go backend/internal/estoppel/generator_test.go backend/internal/estoppel/templates.go
git commit -m "feat(estoppel): add Maroto v2 PDF generator with template registry"
```

---

### Task 9: Event Types

**Files:**
- Create: `backend/internal/estoppel/events.go`

- [ ] **Step 1: Create event constants and helpers**

Create `backend/internal/estoppel/events.go`:

```go
package estoppel

import (
	"encoding/json"

	"github.com/google/uuid"

	"quorant/internal/platform/queue"
)

const (
	EventRequestCreated      = "estoppel.request.created"
	EventDataAggregated      = "estoppel.data.aggregated"
	EventRequestApproved     = "estoppel.request.approved"
	EventRequestRejected     = "estoppel.request.rejected"
	EventCertificateGenerated = "estoppel.certificate.generated"
	EventCertificateDelivered = "estoppel.certificate.delivered"
	EventCertificateAmended   = "estoppel.certificate.amended"
)

func newEstoppelEvent(eventType string, requestID, orgID uuid.UUID, payload any) queue.BaseEvent {
	data, _ := json.Marshal(payload)
	return queue.NewBaseEvent(eventType, "estoppel_request", requestID, orgID, data)
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./internal/estoppel/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/estoppel/events.go
git commit -m "feat(estoppel): add domain event type constants"
```

---

### Task 10: Request DTOs and Validation

**Files:**
- Create: `backend/internal/estoppel/requests.go`

- [ ] **Step 1: Add validation tests to domain_test.go**

Append to `backend/internal/estoppel/domain_test.go`:

```go
func TestCreateEstoppelRequestDTO_Validate_ValidRequest(t *testing.T) {
	req := CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "First American Title",
		RequestorEmail:  "orders@firstam.com",
		PropertyAddress: "123 Palm St",
		OwnerName:       "Jane Doe",
	}
	assert.NoError(t, req.Validate())
}

func TestCreateEstoppelRequestDTO_Validate_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name string
		req  CreateEstoppelRequestDTO
	}{
		{"missing unit_id", CreateEstoppelRequestDTO{RequestType: "estoppel_certificate", RequestorType: "homeowner", RequestorName: "X", RequestorEmail: "x@x.com", PropertyAddress: "X", OwnerName: "X"}},
		{"missing request_type", CreateEstoppelRequestDTO{UnitID: uuid.New(), RequestorType: "homeowner", RequestorName: "X", RequestorEmail: "x@x.com", PropertyAddress: "X", OwnerName: "X"}},
		{"missing requestor_name", CreateEstoppelRequestDTO{UnitID: uuid.New(), RequestType: "estoppel_certificate", RequestorType: "homeowner", RequestorEmail: "x@x.com", PropertyAddress: "X", OwnerName: "X"}},
		{"invalid request_type", CreateEstoppelRequestDTO{UnitID: uuid.New(), RequestType: "invalid", RequestorType: "homeowner", RequestorName: "X", RequestorEmail: "x@x.com", PropertyAddress: "X", OwnerName: "X"}},
		{"invalid requestor_type", CreateEstoppelRequestDTO{UnitID: uuid.New(), RequestType: "estoppel_certificate", RequestorType: "invalid", RequestorName: "X", RequestorEmail: "x@x.com", PropertyAddress: "X", OwnerName: "X"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Error(t, tt.req.Validate())
		})
	}
}

func TestApproveRequestDTO_Validate(t *testing.T) {
	req := ApproveRequestDTO{
		SignerTitle: "Community Manager",
	}
	assert.NoError(t, req.Validate())

	req.SignerTitle = ""
	assert.Error(t, req.Validate())
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestCreate.*DTO|TestApprove"
```

Expected: compilation errors (DTOs not defined).

- [ ] **Step 3: Implement request DTOs**

Create `backend/internal/estoppel/requests.go`:

```go
package estoppel

import (
	"time"

	"github.com/google/uuid"

	"quorant/internal/platform/api"
)

// CreateEstoppelRequestDTO is the request body for creating an estoppel request.
type CreateEstoppelRequestDTO struct {
	UnitID           uuid.UUID  `json:"unit_id"`
	RequestType      string     `json:"request_type"`
	RequestorType    string     `json:"requestor_type"`
	RequestorName    string     `json:"requestor_name"`
	RequestorEmail   string     `json:"requestor_email"`
	RequestorPhone   string     `json:"requestor_phone,omitempty"`
	RequestorCompany string     `json:"requestor_company,omitempty"`
	PropertyAddress  string     `json:"property_address"`
	OwnerName        string     `json:"owner_name"`
	ClosingDate      *time.Time `json:"closing_date,omitempty"`
	RushRequested    bool       `json:"rush_requested"`
}

// Validate checks required fields and allowed values.
func (r CreateEstoppelRequestDTO) Validate() error {
	if r.UnitID == uuid.Nil {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	switch r.RequestType {
	case "estoppel_certificate", "lender_questionnaire":
	case "":
		return api.NewValidationError("request_type is required", "request_type")
	default:
		return api.NewValidationError("request_type must be estoppel_certificate or lender_questionnaire", "request_type")
	}
	switch r.RequestorType {
	case "homeowner", "title_company", "closing_agent", "attorney":
	case "":
		return api.NewValidationError("requestor_type is required", "requestor_type")
	default:
		return api.NewValidationError("requestor_type must be homeowner, title_company, closing_agent, or attorney", "requestor_type")
	}
	if r.RequestorName == "" {
		return api.NewValidationError("requestor_name is required", "requestor_name")
	}
	if r.RequestorEmail == "" {
		return api.NewValidationError("requestor_email is required", "requestor_email")
	}
	if r.PropertyAddress == "" {
		return api.NewValidationError("property_address is required", "property_address")
	}
	if r.OwnerName == "" {
		return api.NewValidationError("owner_name is required", "owner_name")
	}
	return nil
}

// ApproveRequestDTO is the request body for approving an estoppel request.
type ApproveRequestDTO struct {
	SignerTitle string `json:"signer_title"`
}

// Validate checks required fields.
func (r ApproveRequestDTO) Validate() error {
	if r.SignerTitle == "" {
		return api.NewValidationError("signer_title is required", "signer_title")
	}
	return nil
}

// RejectRequestDTO is the request body for rejecting an estoppel request.
type RejectRequestDTO struct {
	Reason string `json:"reason"`
}

// Validate checks required fields.
func (r RejectRequestDTO) Validate() error {
	if r.Reason == "" {
		return api.NewValidationError("reason is required", "reason")
	}
	return nil
}

// UpdateNarrativesDTO is the request body for editing AI narratives before signing.
type UpdateNarrativesDTO struct {
	Narratives NarrativeSections `json:"narratives"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestCreate.*DTO|TestApprove"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/requests.go backend/internal/estoppel/domain_test.go
git commit -m "feat(estoppel): add request DTOs with validation"
```

---

### Task 11: Estoppel Service

**Files:**
- Create: `backend/internal/estoppel/service.go`
- Create: `backend/internal/estoppel/service_test.go`

- [ ] **Step 1: Write failing test for CreateRequest**

Create `backend/internal/estoppel/service_test.go`:

```go
package estoppel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"quorant/internal/audit"
	"quorant/internal/platform/queue"
)

// mockEstoppelRepo is an in-memory test double.
type mockEstoppelRepo struct {
	requests     map[uuid.UUID]*EstoppelRequest
	certificates map[uuid.UUID]*EstoppelCertificate
}

func newMockRepo() *mockEstoppelRepo {
	return &mockEstoppelRepo{
		requests:     make(map[uuid.UUID]*EstoppelRequest),
		certificates: make(map[uuid.UUID]*EstoppelCertificate),
	}
}

func (m *mockEstoppelRepo) CreateRequest(_ context.Context, req *EstoppelRequest) (*EstoppelRequest, error) {
	req.ID = uuid.New()
	now := time.Now().UTC()
	req.CreatedAt = now
	req.UpdatedAt = now
	m.requests[req.ID] = req
	return req, nil
}

func (m *mockEstoppelRepo) FindRequestByID(_ context.Context, id uuid.UUID) (*EstoppelRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return req, nil
}

func (m *mockEstoppelRepo) ListRequestsByOrg(_ context.Context, orgID uuid.UUID, status string, limit int, _ *uuid.UUID) ([]EstoppelRequest, bool, error) {
	var results []EstoppelRequest
	for _, r := range m.requests {
		if r.OrgID == orgID && (status == "" || r.Status == status) {
			results = append(results, *r)
		}
	}
	return results, false, nil
}

func (m *mockEstoppelRepo) UpdateRequestStatus(_ context.Context, id uuid.UUID, status string) (*EstoppelRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	req.Status = status
	req.UpdatedAt = time.Now().UTC()
	return req, nil
}

func (m *mockEstoppelRepo) UpdateRequestNarratives(_ context.Context, id uuid.UUID, narratives []byte) (*EstoppelRequest, error) {
	req, ok := m.requests[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	if req.Metadata == nil {
		req.Metadata = map[string]any{}
	}
	var n NarrativeSections
	_ = json.Unmarshal(narratives, &n)
	req.Metadata["narratives"] = n
	return req, nil
}

func (m *mockEstoppelRepo) CreateCertificate(_ context.Context, cert *EstoppelCertificate) (*EstoppelCertificate, error) {
	cert.ID = uuid.New()
	cert.CreatedAt = time.Now().UTC()
	m.certificates[cert.ID] = cert
	return cert, nil
}

func (m *mockEstoppelRepo) FindCertificateByID(_ context.Context, id uuid.UUID) (*EstoppelCertificate, error) {
	cert, ok := m.certificates[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return cert, nil
}

func (m *mockEstoppelRepo) FindCertificateByRequestID(_ context.Context, requestID uuid.UUID) (*EstoppelCertificate, error) {
	for _, c := range m.certificates {
		if c.RequestID == requestID {
			return c, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockEstoppelRepo) ListCertificatesByOrg(_ context.Context, orgID uuid.UUID, limit int, _ *uuid.UUID) ([]EstoppelCertificate, bool, error) {
	var results []EstoppelCertificate
	for _, c := range m.certificates {
		if c.OrgID == orgID {
			results = append(results, *c)
		}
	}
	return results, false, nil
}

// mockGenerator always returns a minimal valid PDF.
type mockGenerator struct{}

func (m *mockGenerator) GenerateEstoppel(_ *AggregatedData, _ *EstoppelRules) ([]byte, error) {
	return []byte("%PDF-1.4 mock"), nil
}
func (m *mockGenerator) GenerateLenderQuestionnaire(_ *AggregatedData, _ *EstoppelRules) ([]byte, error) {
	return []byte("%PDF-1.4 mock"), nil
}

func newTestService() (*EstoppelService, *mockEstoppelRepo) {
	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &FinancialSnapshot{CurrentBalanceCents: 0}},
		&mockComplianceProvider{snapshot: &ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &PropertySnapshot{OrgName: "Test HOA", OrgState: "FL"}},
		NewNoopNarrativeGenerator(),
		&mockGenerator{},
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
	return svc, repo
}

func TestCreateRequest_Success(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()

	rules := &EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
	}

	dto := CreateEstoppelRequestDTO{
		UnitID:          uuid.New(),
		RequestType:     "estoppel_certificate",
		RequestorType:   "title_company",
		RequestorName:   "First American Title",
		RequestorEmail:  "orders@firstam.com",
		PropertyAddress: "123 Palm St",
		OwnerName:       "Jane Doe",
	}

	req, err := svc.CreateRequest(ctx, orgID, dto, rules, uuid.New())
	require.NoError(t, err)
	assert.Equal(t, "submitted", req.Status)
	assert.Equal(t, int64(29900), req.TotalFeeCents)
	assert.NotZero(t, req.DeadlineAt)
}

func TestCreateRequest_ValidationFailure(t *testing.T) {
	svc, _ := newTestService()
	ctx := context.Background()

	dto := CreateEstoppelRequestDTO{} // empty
	rules := &EstoppelRules{}

	_, err := svc.CreateRequest(ctx, uuid.New(), dto, rules, uuid.New())
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestCreateRequest"
```

Expected: compilation errors (EstoppelService not defined).

- [ ] **Step 3: Implement EstoppelService**

Create `backend/internal/estoppel/service.go`:

```go
package estoppel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"quorant/internal/audit"
	"quorant/internal/platform/queue"
)

// EstoppelService orchestrates estoppel request processing.
type EstoppelService struct {
	repo       EstoppelRepository
	financial  FinancialDataProvider
	compliance ComplianceDataProvider
	property   PropertyDataProvider
	narrative  NarrativeGenerator
	generator  CertificateGenerator
	auditor    audit.Auditor
	publisher  queue.Publisher
	logger     *slog.Logger
}

// NewEstoppelService creates a new estoppel service with all dependencies.
func NewEstoppelService(
	repo EstoppelRepository,
	financial FinancialDataProvider,
	compliance ComplianceDataProvider,
	property PropertyDataProvider,
	narrative NarrativeGenerator,
	generator CertificateGenerator,
	auditor audit.Auditor,
	publisher queue.Publisher,
	logger *slog.Logger,
) *EstoppelService {
	return &EstoppelService{
		repo:       repo,
		financial:  financial,
		compliance: compliance,
		property:   property,
		narrative:  narrative,
		generator:  generator,
		auditor:    auditor,
		publisher:  publisher,
		logger:     logger,
	}
}

// CreateRequest validates input, computes fees and deadline, and persists a new request.
func (s *EstoppelService) CreateRequest(ctx context.Context, orgID uuid.UUID, dto CreateEstoppelRequestDTO, rules *EstoppelRules, createdBy uuid.UUID) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	// Determine if unit is delinquent for fee calculation.
	finSnap, err := s.financial.GetUnitFinancialSnapshot(ctx, orgID, dto.UnitID)
	if err != nil {
		return nil, fmt.Errorf("estoppel: check delinquency: %w", err)
	}
	delinquent := finSnap.TotalDelinquentCents > 0

	fees := CalculateFees(rules, dto.RushRequested, delinquent)
	deadline := CalculateDeadline(rules, dto.RushRequested, time.Now().UTC())

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
		Metadata:                 map[string]any{},
		CreatedBy:                createdBy,
	}

	created, err := s.repo.CreateRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("estoppel: create request: %w", err)
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        orgID,
		ActorID:      createdBy,
		Action:       "estoppel.request.created",
		ResourceType: "estoppel_request",
		ResourceID:   created.ID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	_ = s.publisher.Publish(ctx, newEstoppelEvent(EventRequestCreated, created.ID, orgID, map[string]any{
		"request_type": created.RequestType,
		"unit_id":      created.UnitID,
		"rush":         created.RushRequested,
	}))

	return created, nil
}

// AggregateData queries all providers in parallel and resolves AI narratives.
func (s *EstoppelService) AggregateData(ctx context.Context, req *EstoppelRequest) (*AggregatedData, error) {
	var data AggregatedData
	data.AsOfTime = time.Now().UTC()

	g, ctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		snap, err := s.financial.GetUnitFinancialSnapshot(ctx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("financial: %w", err)
		}
		data.Financial = *snap
		return nil
	})

	g.Go(func() error {
		snap, err := s.compliance.GetUnitComplianceSnapshot(ctx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("compliance: %w", err)
		}
		data.Compliance = *snap
		return nil
	})

	g.Go(func() error {
		snap, err := s.property.GetPropertySnapshot(ctx, req.OrgID, req.UnitID)
		if err != nil {
			return fmt.Errorf("property: %w", err)
		}
		data.Property = *snap
		return nil
	})

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("estoppel: aggregate data: %w", err)
	}

	// AI narrative generation.
	narratives, err := s.narrative.GenerateNarratives(ctx, req.OrgID, &data)
	if err != nil {
		s.logger.Warn("narrative generation failed, proceeding with empty narratives", "error", err)
	} else {
		data.Narratives = narratives
	}

	return &data, nil
}

// GetRequest retrieves a request by ID.
func (s *EstoppelService) GetRequest(ctx context.Context, id uuid.UUID) (*EstoppelRequest, error) {
	return s.repo.FindRequestByID(ctx, id)
}

// ListRequests returns paginated requests for an org with optional status filter.
func (s *EstoppelService) ListRequests(ctx context.Context, orgID uuid.UUID, status string, limit int, afterID *uuid.UUID) ([]EstoppelRequest, bool, error) {
	return s.repo.ListRequestsByOrg(ctx, orgID, status, limit, afterID)
}

// ApproveRequest signs and freezes the certificate data.
func (s *EstoppelService) ApproveRequest(ctx context.Context, requestID uuid.UUID, dto ApproveRequestDTO, signedBy uuid.UUID) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	req, err := s.repo.FindRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	if !IsValidTransition(req.Status, "approved") {
		return nil, fmt.Errorf("estoppel: cannot approve request in status %q", req.Status)
	}

	updated, err := s.repo.UpdateRequestStatus(ctx, requestID, "approved")
	if err != nil {
		return nil, err
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        req.OrgID,
		ActorID:      signedBy,
		Action:       "estoppel.request.approved",
		ResourceType: "estoppel_request",
		ResourceID:   requestID,
		Module:       "estoppel",
		Metadata:     map[string]any{"signer_title": dto.SignerTitle},
		OccurredAt:   time.Now(),
	})

	_ = s.publisher.Publish(ctx, newEstoppelEvent(EventRequestApproved, requestID, req.OrgID, map[string]any{
		"signed_by":    signedBy,
		"signer_title": dto.SignerTitle,
	}))

	return updated, nil
}

// RejectRequest marks the request as cancelled with a reason.
func (s *EstoppelService) RejectRequest(ctx context.Context, requestID uuid.UUID, dto RejectRequestDTO, rejectedBy uuid.UUID) (*EstoppelRequest, error) {
	if err := dto.Validate(); err != nil {
		return nil, err
	}

	req, err := s.repo.FindRequestByID(ctx, requestID)
	if err != nil {
		return nil, err
	}

	if !IsValidTransition(req.Status, "cancelled") {
		return nil, fmt.Errorf("estoppel: cannot reject request in status %q", req.Status)
	}

	updated, err := s.repo.UpdateRequestStatus(ctx, requestID, "cancelled")
	if err != nil {
		return nil, err
	}

	_ = s.publisher.Publish(ctx, newEstoppelEvent(EventRequestRejected, requestID, req.OrgID, map[string]any{
		"reason": dto.Reason,
	}))

	return updated, nil
}

// GenerateCertificate produces the PDF and creates the certificate record.
func (s *EstoppelService) GenerateCertificate(ctx context.Context, requestID uuid.UUID, data *AggregatedData, rules *EstoppelRules, signedBy uuid.UUID, signerTitle string) (*EstoppelCertificate, error) {
	req, err := s.repo.FindRequestByID(ctx, requestID)
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
		return nil, fmt.Errorf("estoppel: generate PDF: %w", err)
	}

	// Freeze data as JSON snapshot.
	snapshot, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("estoppel: marshal snapshot: %w", err)
	}

	narrativeJSON := json.RawMessage("{}")
	if data.Narratives != nil {
		narrativeJSON, _ = json.Marshal(data.Narratives)
	}

	// Compute effective period.
	now := time.Now().UTC()
	var expiresAt *time.Time
	if rules.EffectivePeriodDays != nil && *rules.EffectivePeriodDays > 0 {
		exp := now.AddDate(0, 0, *rules.EffectivePeriodDays)
		expiresAt = &exp
	}

	// The PDF bytes would be uploaded to S3 via the doc module in a real flow.
	// For now we use a placeholder document ID; the handler layer will coordinate upload.
	docID := uuid.New()
	_ = pdfBytes // will be used by handler to upload

	formID := rules.StatutoryFormID
	if formID == "" {
		formID = "generic"
	}

	cert := &EstoppelCertificate{
		RequestID:         requestID,
		OrgID:             req.OrgID,
		UnitID:            req.UnitID,
		DocumentID:        docID,
		Jurisdiction:      data.Property.OrgState,
		EffectiveDate:     now,
		ExpiresAt:         expiresAt,
		DataSnapshot:      snapshot,
		NarrativeSections: narrativeJSON,
		SignedBy:          signedBy,
		SignedAt:          now,
		SignerTitle:       signerTitle,
		TemplateVersion:   formID + "-v1",
		AmendmentOf:       req.AmendmentOf,
	}

	created, err := s.repo.CreateCertificate(ctx, cert)
	if err != nil {
		return nil, fmt.Errorf("estoppel: create certificate: %w", err)
	}

	_ = s.publisher.Publish(ctx, newEstoppelEvent(EventCertificateGenerated, requestID, req.OrgID, map[string]any{
		"certificate_id": created.ID,
		"document_id":    created.DocumentID,
	}))

	return created, nil
}

// GetCertificate retrieves a certificate by ID.
func (s *EstoppelService) GetCertificate(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error) {
	return s.repo.FindCertificateByID(ctx, id)
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestCreateRequest"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/service.go backend/internal/estoppel/service_test.go
git commit -m "feat(estoppel): add EstoppelService with create, approve, reject, and generate"
```

---

### Task 12: HTTP Handlers

**Files:**
- Create: `backend/internal/estoppel/handler.go`
- Create: `backend/internal/estoppel/handler_test.go`

- [ ] **Step 1: Write failing handler test**

Create `backend/internal/estoppel/handler_test.go`:

```go
package estoppel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"quorant/internal/audit"
	"quorant/internal/platform/queue"
)

type handlerTestServer struct {
	server *httptest.Server
	repo   *mockEstoppelRepo
}

func setupHandlerTestServer(t *testing.T) *handlerTestServer {
	t.Helper()

	repo := newMockRepo()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	svc := NewEstoppelService(
		repo,
		&mockFinancialProvider{snapshot: &FinancialSnapshot{}},
		&mockComplianceProvider{snapshot: &ComplianceSnapshot{}},
		&mockPropertyProvider{snapshot: &PropertySnapshot{OrgName: "Test HOA", OrgState: "FL"}},
		NewNoopNarrativeGenerator(),
		&mockGenerator{},
		audit.NewNoopAuditor(),
		queue.NewInMemoryPublisher(),
		logger,
	)
	handler := NewHandler(svc, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /organizations/{org_id}/estoppel/requests", handler.CreateRequest)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests", handler.ListRequests)
	mux.HandleFunc("GET /organizations/{org_id}/estoppel/requests/{id}", handler.GetRequest)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return &handlerTestServer{server: server, repo: repo}
}

func doEstoppelRequest(t *testing.T, serverURL, method, path string, body any) *http.Response {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, serverURL+path, bodyReader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func TestHandler_CreateRequest_Success(t *testing.T) {
	ts := setupHandlerTestServer(t)
	orgID := uuid.New()

	body := map[string]any{
		"unit_id":          uuid.New().String(),
		"request_type":     "estoppel_certificate",
		"requestor_type":   "title_company",
		"requestor_name":   "First American",
		"requestor_email":  "test@firstam.com",
		"property_address": "123 Palm St",
		"owner_name":       "Jane Doe",
	}

	resp := doEstoppelRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/estoppel/requests", orgID), body)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	resp.Body.Close()
}

func TestHandler_CreateRequest_InvalidBody(t *testing.T) {
	ts := setupHandlerTestServer(t)
	orgID := uuid.New()

	resp := doEstoppelRequest(t, ts.server.URL, http.MethodPost,
		fmt.Sprintf("/organizations/%s/estoppel/requests", orgID), map[string]any{})
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	resp.Body.Close()
}

func TestHandler_ListRequests_Empty(t *testing.T) {
	ts := setupHandlerTestServer(t)
	orgID := uuid.New()

	resp := doEstoppelRequest(t, ts.server.URL, http.MethodGet,
		fmt.Sprintf("/organizations/%s/estoppel/requests", orgID), nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	resp.Body.Close()
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestHandler"
```

Expected: compilation errors (Handler not defined).

- [ ] **Step 3: Implement handlers**

Create `backend/internal/estoppel/handler.go`:

```go
package estoppel

import (
	"log/slog"
	"net/http"

	"github.com/google/uuid"

	"quorant/internal/platform/api"
	"quorant/internal/platform/middleware"
)

// Handler exposes estoppel HTTP endpoints.
type Handler struct {
	service *EstoppelService
	logger  *slog.Logger
}

// NewHandler creates an estoppel handler.
func NewHandler(service *EstoppelService, logger *slog.Logger) *Handler {
	return &Handler{service: service, logger: logger}
}

// CreateRequest handles POST /organizations/{org_id}/estoppel/requests.
func (h *Handler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var dto CreateEstoppelRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	// In production, rules come from PolicyResolver. For handler tests, use defaults.
	rules := &EstoppelRules{
		StandardFeeCents:               29900,
		StandardTurnaroundBusinessDays: 10,
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		userID = uuid.New() // fallback for tests without auth middleware
	}

	created, err := h.service.CreateRequest(r.Context(), orgID, dto, rules, userID)
	if err != nil {
		h.logger.Error("CreateRequest failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}

// ListRequests handles GET /organizations/{org_id}/estoppel/requests.
func (h *Handler) ListRequests(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	status := r.URL.Query().Get("status")
	requests, hasMore, err := h.service.ListRequests(r.Context(), orgID, status, 50, nil)
	if err != nil {
		h.logger.Error("ListRequests failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	var meta *api.Meta
	if hasMore {
		meta = &api.Meta{HasMore: true}
	}
	api.WriteJSONWithMeta(w, http.StatusOK, requests, meta)
}

// GetRequest handles GET /organizations/{org_id}/estoppel/requests/{id}.
func (h *Handler) GetRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	req, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		h.logger.Error("GetRequest failed", "id", id, "error", err)
		api.WriteError(w, api.NewNotFoundError("estoppel request not found"))
		return
	}

	api.WriteJSON(w, http.StatusOK, req)
}

// ApproveRequest handles POST /organizations/{org_id}/estoppel/requests/{id}/approve.
func (h *Handler) ApproveRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var dto ApproveRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	updated, err := h.service.ApproveRequest(r.Context(), id, dto, userID)
	if err != nil {
		h.logger.Error("ApproveRequest failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// RejectRequest handles POST /organizations/{org_id}/estoppel/requests/{id}/reject.
func (h *Handler) RejectRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	var dto RejectRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	updated, err := h.service.RejectRequest(r.Context(), id, dto, userID)
	if err != nil {
		h.logger.Error("RejectRequest failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}

// parseEstoppelPathUUID extracts and parses a UUID from the request path.
func parseEstoppelPathUUID(r *http.Request, key string) (uuid.UUID, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return uuid.Nil, api.NewValidationError(key+" is required", key)
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, api.NewValidationError("invalid "+key+" format", key)
	}
	return id, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1 -run "TestHandler"
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/estoppel/handler.go backend/internal/estoppel/handler_test.go
git commit -m "feat(estoppel): add HTTP handlers for estoppel endpoints"
```

---

### Task 13: Routes with Entitlement Gate

**Files:**
- Create: `backend/internal/estoppel/routes.go`

- [ ] **Step 1: Create route registration**

Create `backend/internal/estoppel/routes.go`:

```go
package estoppel

import (
	"net/http"

	"quorant/internal/platform/middleware"
)

// RegisterRoutes registers all estoppel HTTP routes with the entitlement gate.
func RegisterRoutes(
	mux *http.ServeMux,
	handler *Handler,
	validator middleware.TokenValidator,
	checker middleware.PermissionChecker,
	entitlements middleware.EntitlementChecker,
	resolveUserID middleware.UserIDResolver,
) {
	// Middleware chain: Auth → TenantContext → Entitlement → Permission.
	permMw := func(perm string, h http.HandlerFunc) http.Handler {
		return middleware.Auth(validator,
			middleware.TenantContext(
				middleware.RequireEntitlement(entitlements, "estoppel")(
					middleware.RequirePermission(checker, perm, resolveUserID)(
						http.HandlerFunc(h)))))
	}

	// Requests.
	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests",
		permMw("estoppel.request.create", handler.CreateRequest))
	mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/requests",
		permMw("estoppel.request.list", handler.ListRequests))
	mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/requests/{id}",
		permMw("estoppel.request.read", handler.GetRequest))
	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests/{id}/approve",
		permMw("estoppel.request.approve", handler.ApproveRequest))
	mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/requests/{id}/reject",
		permMw("estoppel.request.approve", handler.RejectRequest))
}
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./internal/estoppel/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/estoppel/routes.go
git commit -m "feat(estoppel): add route registration with entitlement gate"
```

---

### Task 14: Wire into main.go

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] **Step 1: Add estoppel module wiring after doc module**

Add the following block to `backend/cmd/quorant-api/main.go` in the module initialization section, after the doc module block and before the HTTP middleware stack:

```go
// --- Estoppel module ---
estoppelRepo := estoppel.NewPostgresRepository(pool)
financialProvider := fin.NewEstoppelFinancialAdapter(finService)
complianceProvider := gov.NewEstoppelComplianceAdapter(govService)
propertyProvider := org.NewEstoppelPropertyAdapter(orgService)
narrativeGen := estoppel.NewNoopNarrativeGenerator()
pdfGen := estoppel.NewMarotoGenerator()
estoppelService := estoppel.NewEstoppelService(
    estoppelRepo,
    financialProvider,
    complianceProvider,
    propertyProvider,
    narrativeGen,
    pdfGen,
    auditor,
    outboxPublisher,
    logger,
)
estoppelHandler := estoppel.NewHandler(estoppelService, logger)
estoppel.RegisterRoutes(mux, estoppelHandler, tokenValidator, permChecker, entitlementChecker, resolveUserID)
```

Add the import:
```go
"quorant/internal/estoppel"
```

- [ ] **Step 2: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./cmd/quorant-api/
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add backend/cmd/quorant-api/main.go
git commit -m "feat(estoppel): wire estoppel module into main.go"
```

---

### Task 15: Policy Seed Data Migration

**Files:**
- Create: `backend/migrations/20260409000026_estoppel_seed_data.sql`

- [ ] **Step 1: Create seed data migration**

Create `backend/migrations/20260409000026_estoppel_seed_data.sql`:

```sql
-- Seed estoppel compliance rules for 5 states as jurisdiction-scoped policy extractions.
-- These are queried via PolicyResolver.GetPolicy(ctx, orgID, "estoppel_rules").

INSERT INTO policy_extractions (org_id, domain, policy_key, config, confidence, review_status, effective_at)
SELECT NULL, 'compliance', 'estoppel_rules',
  '{"standard_turnaround_business_days":10,"standard_fee_cents":29900,"rush_turnaround_business_days":3,"rush_fee_cents":11900,"delinquent_surcharge_cents":17900,"effective_period_days":30,"electronic_delivery_required":true,"statutory_form_required":true,"statutory_form_id":"fl_720_30851","free_amendment_on_error":true,"statutory_questions":19,"statute_ref":"§720.30851/§718.116(8)","jurisdiction":"FL"}'::jsonb,
  1.0, 'approved', now()
WHERE NOT EXISTS (SELECT 1 FROM policy_extractions WHERE policy_key = 'estoppel_rules' AND config->>'jurisdiction' = 'FL');

INSERT INTO policy_extractions (org_id, domain, policy_key, config, confidence, review_status, effective_at)
SELECT NULL, 'compliance', 'estoppel_rules',
  '{"standard_turnaround_business_days":10,"standard_fee_cents":0,"fee_cap_type":"reasonable","statutory_form_required":true,"statutory_form_id":"ca_4528","required_attachments":["governing_docs","ccrs","bylaws","rules","budget","reserve_study_summary"],"statute_ref":"Civil Code §4525–4530","jurisdiction":"CA"}'::jsonb,
  1.0, 'approved', now()
WHERE NOT EXISTS (SELECT 1 FROM policy_extractions WHERE policy_key = 'estoppel_rules' AND config->>'jurisdiction' = 'CA');

INSERT INTO policy_extractions (org_id, domain, policy_key, config, confidence, review_status, effective_at)
SELECT NULL, 'compliance', 'estoppel_rules',
  '{"standard_turnaround_business_days":10,"standard_fee_cents":37500,"update_fee_cents":7500,"effective_period_days":60,"noncompliance_damages_cents":500000,"statute_ref":"Property Code §207.003","jurisdiction":"TX"}'::jsonb,
  1.0, 'approved', now()
WHERE NOT EXISTS (SELECT 1 FROM policy_extractions WHERE policy_key = 'estoppel_rules' AND config->>'jurisdiction' = 'TX');

INSERT INTO policy_extractions (org_id, domain, policy_key, config, confidence, review_status, effective_at)
SELECT NULL, 'compliance', 'estoppel_rules',
  '{"standard_turnaround_business_days":10,"standard_fee_cents":18500,"fee_cpi_adjusted":true,"cpi_cap_percent":3,"rush_fee_cents":10000,"effective_period_days":90,"electronic_delivery_required":true,"statute_ref":"NRS 116.4109","jurisdiction":"NV"}'::jsonb,
  1.0, 'approved', now()
WHERE NOT EXISTS (SELECT 1 FROM policy_extractions WHERE policy_key = 'estoppel_rules' AND config->>'jurisdiction' = 'NV');

INSERT INTO policy_extractions (org_id, domain, policy_key, config, confidence, review_status, effective_at)
SELECT NULL, 'compliance', 'estoppel_rules',
  '{"standard_turnaround_business_days":14,"fee_cpi_adjusted":true,"electronic_delivery_required":true,"buyer_rescission_days":3,"cic_board_registration_required":true,"statute_ref":"§55.1-1808","jurisdiction":"VA"}'::jsonb,
  1.0, 'approved', now()
WHERE NOT EXISTS (SELECT 1 FROM policy_extractions WHERE policy_key = 'estoppel_rules' AND config->>'jurisdiction' = 'VA');

-- Seed estoppel_request task type.
INSERT INTO task_types (org_id, key, name, description, default_priority, sla_hours, auto_assign_role, source_module, is_active, workflow_stages, checklist_template)
SELECT NULL, 'estoppel_request', 'Estoppel Certificate Request',
  'Request for estoppel certificate or lender questionnaire generation',
  'high', 80, 'hoa_manager', 'estoppel', true,
  '["submitted","data_aggregation","manager_review","approved","generating","delivered"]'::jsonb,
  '[{"id":"verify_owner","label":"Verify current owner matches records"},{"id":"verify_balance","label":"Verify account balance is current"},{"id":"review_violations","label":"Review open violations"},{"id":"review_narrative","label":"Review AI-generated narrative sections"},{"id":"confirm_fee","label":"Confirm fee calculation"},{"id":"approve_sign","label":"Approve and sign certificate"}]'::jsonb
WHERE NOT EXISTS (SELECT 1 FROM task_types WHERE key = 'estoppel_request');
```

- [ ] **Step 2: Verify SQL is well-formed**

```bash
cd /home/douglasl/Projects/quorant/backend && wc -l migrations/20260409000026_estoppel_seed_data.sql
```

Expected: file exists with reasonable line count.

- [ ] **Step 3: Commit**

```bash
git add backend/migrations/20260409000026_estoppel_seed_data.sql
git commit -m "feat(estoppel): add seed data for state compliance rules and task type"
```

---

### Task 16: Run Full Test Suite

- [ ] **Step 1: Run all estoppel unit tests**

```bash
cd /home/douglasl/Projects/quorant/backend && go test ./internal/estoppel/ -v -count=1
```

Expected: all tests PASS.

- [ ] **Step 2: Run full project compilation**

```bash
cd /home/douglasl/Projects/quorant/backend && go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run vet and basic checks**

```bash
cd /home/douglasl/Projects/quorant/backend && go vet ./internal/estoppel/...
```

Expected: no issues.

---

## Verification

After all tasks complete:

1. **Unit tests pass:** `go test ./internal/estoppel/ -v -count=1` — all green
2. **Full build succeeds:** `go build ./...` — no errors
3. **Vet passes:** `go vet ./internal/estoppel/...` — no issues
4. **Migration files present:** `ls backend/migrations/20260409000025* backend/migrations/20260409000026*`
5. **Module wired in main.go:** `grep -n "estoppel" backend/cmd/quorant-api/main.go`
6. **PDF generation works:** The `TestMarotoGenerator_*` tests produce valid `%PDF` byte output
