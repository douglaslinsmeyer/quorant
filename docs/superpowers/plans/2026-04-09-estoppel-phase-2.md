# Estoppel Phase 2 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Complete the estoppel module by wiring document storage, adding 4 missing endpoints, integrating jurisdiction-scoped compliance rules, and differentiating state-specific PDF templates.

**Architecture:** Extend DocService with server-side upload, define a DocumentUploader interface in the estoppel package to avoid circular imports, add a JurisdictionRulesRepository to query the `jurisdiction_rules` table, and expand the template registry with 5 state-specific builders.

**Tech Stack:** Go, pgx/v5, Maroto v2, MinIO/S3, existing platform packages.

**Spec:** `docs/superpowers/specs/2026-04-09-estoppel-phase-2-design.md`

---

## File Structure

```
backend/internal/doc/
├── service.go              # MODIFY: add UploadFromBytes method
└── service_test.go         # MODIFY: add test for UploadFromBytes

backend/internal/estoppel/
├── doc_adapter.go          # CREATE: DocumentUploader interface + DocService adapter
├── jurisdiction_rules.go   # CREATE: JurisdictionRulesRepository interface + Postgres impl
├── jurisdiction_rules_test.go # CREATE: integration tests
├── service.go              # MODIFY: add docUploader field, ResolveRules, GeneratePreviewPDF
├── service_test.go         # MODIFY: tests for new methods
├── handler.go              # MODIFY: add 4 new handlers, replace defaultEstoppelRules
├── handler_test.go         # MODIFY: tests for new handlers
├── routes.go               # MODIFY: add 4 new routes
├── requests.go             # MODIFY: add AmendCertificateDTO
└── templates.go            # MODIFY: add 5 state-specific template builders

backend/cmd/quorant-api/main.go # MODIFY: wire new dependencies
```

---

### Task 1: Extend DocService with UploadFromBytes

**Files:**
- Modify: `backend/internal/doc/service.go`
- Modify: `backend/internal/doc/domain.go`

- [ ] **Step 1: Add UploadFromBytesRequest to domain.go**

Add after line 49 of `backend/internal/doc/domain.go`:

```go
// UploadFromBytesRequest carries metadata for server-side document uploads
// where the file content is already in memory (e.g., generated PDFs).
type UploadFromBytesRequest struct {
	Title       string     `json:"title"`
	FileName    string     `json:"file_name"`
	ContentType string     `json:"content_type"`
	CategoryID  *uuid.UUID `json:"category_id,omitempty"`
	Visibility  string     `json:"visibility,omitempty"`
}

// Validate checks required fields for a server-side upload.
func (r UploadFromBytesRequest) Validate() error {
	if r.Title == "" {
		return api.NewValidationError("title is required", "title")
	}
	if r.FileName == "" {
		return api.NewValidationError("file_name is required", "file_name")
	}
	if r.ContentType == "" {
		return api.NewValidationError("content_type is required", "content_type")
	}
	return nil
}
```

- [ ] **Step 2: Add UploadFromBytes method to DocService**

Add to `backend/internal/doc/service.go` after the `UploadDocument` method (after line 65):

```go
// UploadFromBytes stores file bytes directly to S3 and creates a document
// record in a single operation. Use this for server-generated content (e.g.,
// PDF certificates) where the file is already in memory.
func (s *DocService) UploadFromBytes(ctx context.Context, orgID uuid.UUID, req UploadFromBytesRequest, data []byte, uploadedBy uuid.UUID) (*Document, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	visibility := req.Visibility
	if visibility == "" {
		visibility = "members"
	}

	// Generate a unique storage key.
	storageKey := fmt.Sprintf("generated/%s/%s/%s", orgID, uuid.New(), req.FileName)

	// Upload bytes to S3.
	reader := bytes.NewReader(data)
	if err := s.storage.Upload(ctx, s.bucket, storageKey, req.ContentType, reader, int64(len(data))); err != nil {
		return nil, fmt.Errorf("doc: UploadFromBytes: upload to storage: %w", err)
	}

	doc := &Document{
		OrgID:       orgID,
		CategoryID:  req.CategoryID,
		UploadedBy:  uploadedBy,
		Title:       req.Title,
		FileName:    req.FileName,
		ContentType: req.ContentType,
		SizeBytes:   int64(len(data)),
		StorageKey:  storageKey,
		Visibility:  visibility,
		Metadata:    map[string]any{},
	}

	return s.repo.Create(ctx, doc)
}
```

Add `"bytes"` and `"fmt"` to the import block if not already present.

- [ ] **Step 3: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go build ./internal/doc/
```

- [ ] **Step 4: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/doc/service.go backend/internal/doc/domain.go && git commit -m "feat(doc): add UploadFromBytes for server-side document upload

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: DocumentUploader Interface and Adapter

**Files:**
- Create: `backend/internal/estoppel/doc_adapter.go`

- [ ] **Step 1: Create the interface and adapter**

Create `backend/internal/estoppel/doc_adapter.go`:

```go
package estoppel

import (
	"context"

	"github.com/google/uuid"
)

// DocumentUploader stores generated documents (PDFs) and returns the document
// ID. Defined in the estoppel package to avoid importing the doc package
// directly into the service.
type DocumentUploader interface {
	UploadFromBytes(ctx context.Context, orgID uuid.UUID, title, fileName, contentType string, data []byte, uploadedBy uuid.UUID) (documentID uuid.UUID, err error)
}
```

- [ ] **Step 2: Create the doc module adapter**

Create `backend/internal/doc/estoppel_adapter.go`:

```go
package doc

import (
	"context"

	"github.com/google/uuid"
)

// EstoppelDocumentAdapter implements estoppel.DocumentUploader by wrapping
// DocService.UploadFromBytes.
type EstoppelDocumentAdapter struct {
	service *DocService
}

// NewEstoppelDocumentAdapter returns an adapter for the estoppel module.
func NewEstoppelDocumentAdapter(service *DocService) *EstoppelDocumentAdapter {
	return &EstoppelDocumentAdapter{service: service}
}

// UploadFromBytes uploads file bytes via DocService and returns the document ID.
func (a *EstoppelDocumentAdapter) UploadFromBytes(ctx context.Context, orgID uuid.UUID, title, fileName, contentType string, data []byte, uploadedBy uuid.UUID) (uuid.UUID, error) {
	req := UploadFromBytesRequest{
		Title:       title,
		FileName:    fileName,
		ContentType: contentType,
		Visibility:  "board_only",
	}
	doc, err := a.service.UploadFromBytes(ctx, orgID, req, data, uploadedBy)
	if err != nil {
		return uuid.Nil, err
	}
	return doc.ID, nil
}
```

- [ ] **Step 3: Verify compilation**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go build ./internal/estoppel/ ./internal/doc/
```

- [ ] **Step 4: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/doc_adapter.go backend/internal/doc/estoppel_adapter.go && git commit -m "feat(estoppel): add DocumentUploader interface and doc adapter

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Wire DocumentUploader into EstoppelService

**Files:**
- Modify: `backend/internal/estoppel/service.go`
- Modify: `backend/internal/estoppel/service_test.go`

- [ ] **Step 1: Write failing test for document upload in GenerateCertificate**

Add to `backend/internal/estoppel/service_test.go`:

```go
// mockDocUploader implements DocumentUploader for testing.
type mockDocUploader struct {
	lastData []byte
	docID    uuid.UUID
}

func (m *mockDocUploader) UploadFromBytes(_ context.Context, _ uuid.UUID, _, _, _ string, data []byte, _ uuid.UUID) (uuid.UUID, error) {
	m.lastData = data
	if m.docID == uuid.Nil {
		m.docID = uuid.New()
	}
	return m.docID, nil
}

func TestGenerateCertificate_UploadsDocument(t *testing.T) {
	svc, repo := newTestService()
	// Attach doc uploader to service.
	docUploader := &mockDocUploader{}
	svc.docUploader = docUploader

	ctx := context.Background()
	orgID := uuid.New()

	// Create a request first.
	rules := &EstoppelRules{StandardFeeCents: 29900, StandardTurnaroundBusinessDays: 10}
	dto := CreateEstoppelRequestDTO{
		UnitID: uuid.New(), RequestType: "estoppel_certificate",
		RequestorType: "title_company", RequestorName: "Test",
		RequestorEmail: "t@t.com", PropertyAddress: "123 St", OwnerName: "Jane",
	}
	req, err := svc.CreateRequest(ctx, orgID, dto, rules, uuid.New())
	require.NoError(t, err)

	// Approve the request.
	_, err = repo.UpdateRequestStatus(ctx, req.ID, "manager_review")
	require.NoError(t, err)
	_, err = svc.ApproveRequest(ctx, req.ID, ApproveRequestDTO{SignerTitle: "Manager"}, uuid.New())
	require.NoError(t, err)

	// Generate certificate.
	data := newTestAggregatedData()
	cert, err := svc.GenerateCertificate(ctx, req.ID, data, rules, uuid.New(), "Manager")
	require.NoError(t, err)
	require.NotNil(t, cert)

	// DocumentID should be populated.
	require.NotNil(t, cert.DocumentID, "DocumentID should be set after upload")
	assert.Equal(t, docUploader.docID, *cert.DocumentID)
	assert.NotEmpty(t, docUploader.lastData, "PDF bytes should have been uploaded")
}
```

- [ ] **Step 2: Add docUploader field to EstoppelService**

In `backend/internal/estoppel/service.go`, add `docUploader DocumentUploader` to the struct and constructor:

```go
type EstoppelService struct {
	repo        EstoppelRepository
	financial   FinancialDataProvider
	compliance  ComplianceDataProvider
	property    PropertyDataProvider
	narrative   NarrativeGenerator
	generator   CertificateGenerator
	docUploader DocumentUploader
	auditor     audit.Auditor
	publisher   queue.Publisher
	logger      *slog.Logger
}

func NewEstoppelService(
	repo EstoppelRepository,
	financial FinancialDataProvider,
	compliance ComplianceDataProvider,
	property PropertyDataProvider,
	narrative NarrativeGenerator,
	generator CertificateGenerator,
	docUploader DocumentUploader,
	auditor audit.Auditor,
	publisher queue.Publisher,
	logger *slog.Logger,
) *EstoppelService {
	return &EstoppelService{
		repo:        repo,
		financial:   financial,
		compliance:  compliance,
		property:    property,
		narrative:   narrative,
		generator:   generator,
		docUploader: docUploader,
		auditor:     auditor,
		publisher:   publisher,
		logger:      logger,
	}
}
```

- [ ] **Step 3: Update GenerateCertificate to upload PDF and set DocumentID**

In `GenerateCertificate`, replace the line `_ = pdfBytes` with:

```go
	// Upload PDF to document storage.
	var docID *uuid.UUID
	if s.docUploader != nil {
		fileName := fmt.Sprintf("estoppel-%s.pdf", requestID)
		title := fmt.Sprintf("Estoppel Certificate - %s", req.OwnerName)
		if req.RequestType == "lender_questionnaire" {
			title = fmt.Sprintf("Lender Questionnaire - %s", req.OwnerName)
		}
		id, uploadErr := s.docUploader.UploadFromBytes(ctx, req.OrgID, title, fileName, "application/pdf", pdfBytes, signedBy)
		if uploadErr != nil {
			return nil, fmt.Errorf("uploading PDF: %w", uploadErr)
		}
		docID = &id
	}
```

And set `DocumentID: docID` on the certificate struct.

- [ ] **Step 4: Update newTestService to include nil docUploader**

In `service_test.go`, update `newTestService()` to pass `nil` for `docUploader` (tests that don't need upload will use nil; the test above sets it explicitly).

- [ ] **Step 5: Fix all existing callers of NewEstoppelService**

Update `handler_test.go`'s test server setup and any other callers to pass the new `docUploader` parameter (nil for handler tests).

- [ ] **Step 6: Run tests**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
```

- [ ] **Step 7: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/service.go backend/internal/estoppel/service_test.go backend/internal/estoppel/handler_test.go && git commit -m "feat(estoppel): wire DocumentUploader into service and GenerateCertificate

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: JurisdictionRulesRepository

**Files:**
- Create: `backend/internal/estoppel/jurisdiction_rules.go`
- Create: `backend/internal/estoppel/jurisdiction_rules_test.go`

- [ ] **Step 1: Write failing test**

Create `backend/internal/estoppel/jurisdiction_rules_test.go`:

```go
package estoppel

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockJurisdictionRules_ReturnsRulesForKnownState(t *testing.T) {
	repo := &mockJurisdictionRulesRepo{
		rules: map[string]*EstoppelRules{
			"FL": {
				StandardFeeCents:               29900,
				StandardTurnaroundBusinessDays: 10,
				StatutoryFormID:                "fl_720_30851",
				StatuteRef:                     "§720.30851/§718.116(8)",
			},
		},
	}
	rules, err := repo.GetEstoppelRules(context.Background(), "FL")
	require.NoError(t, err)
	require.NotNil(t, rules)
	assert.Equal(t, int64(29900), rules.StandardFeeCents)
	assert.Equal(t, "fl_720_30851", rules.StatutoryFormID)
}

func TestMockJurisdictionRules_ReturnsNilForUnknownState(t *testing.T) {
	repo := &mockJurisdictionRulesRepo{rules: map[string]*EstoppelRules{}}
	rules, err := repo.GetEstoppelRules(context.Background(), "ZZ")
	require.NoError(t, err)
	assert.Nil(t, rules)
}
```

- [ ] **Step 2: Implement interface, mock, and Postgres implementation**

Create `backend/internal/estoppel/jurisdiction_rules.go`:

```go
package estoppel

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// JurisdictionRulesRepository retrieves state-specific estoppel compliance
// rules from the jurisdiction_rules table.
type JurisdictionRulesRepository interface {
	GetEstoppelRules(ctx context.Context, jurisdiction string) (*EstoppelRules, error)
}

// PostgresJurisdictionRulesRepository queries jurisdiction_rules using pgx.
type PostgresJurisdictionRulesRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresJurisdictionRulesRepository creates a new repository.
func NewPostgresJurisdictionRulesRepository(pool *pgxpool.Pool) *PostgresJurisdictionRulesRepository {
	return &PostgresJurisdictionRulesRepository{pool: pool}
}

// GetEstoppelRules returns the active estoppel rules for the given state code,
// or nil if no rules are configured.
func (r *PostgresJurisdictionRulesRepository) GetEstoppelRules(ctx context.Context, jurisdiction string) (*EstoppelRules, error) {
	const q = `
		SELECT value FROM jurisdiction_rules
		WHERE jurisdiction = $1
		  AND rule_category = 'estoppel'
		  AND rule_key = 'estoppel_rules'
		  AND (expiration_date IS NULL OR expiration_date > now())
		ORDER BY effective_date DESC
		LIMIT 1`

	var valueJSON []byte
	err := r.pool.QueryRow(ctx, q, jurisdiction).Scan(&valueJSON)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("estoppel: GetEstoppelRules: %w", err)
	}

	var rules EstoppelRules
	if err := json.Unmarshal(valueJSON, &rules); err != nil {
		return nil, fmt.Errorf("estoppel: GetEstoppelRules unmarshal: %w", err)
	}
	return &rules, nil
}

// mockJurisdictionRulesRepo is a test double.
type mockJurisdictionRulesRepo struct {
	rules map[string]*EstoppelRules
}

func (m *mockJurisdictionRulesRepo) GetEstoppelRules(_ context.Context, jurisdiction string) (*EstoppelRules, error) {
	r, ok := m.rules[jurisdiction]
	if !ok {
		return nil, nil
	}
	return r, nil
}
```

- [ ] **Step 3: Run tests**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1 -run "TestMockJurisdiction"
```

- [ ] **Step 4: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/jurisdiction_rules.go backend/internal/estoppel/jurisdiction_rules_test.go && git commit -m "feat(estoppel): add JurisdictionRulesRepository for state compliance lookup

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: Wire PolicyResolver into Handler

**Files:**
- Modify: `backend/internal/estoppel/service.go`
- Modify: `backend/internal/estoppel/handler.go`
- Modify: `backend/internal/estoppel/service_test.go`

- [ ] **Step 1: Add ResolveRules method to service**

Add to `backend/internal/estoppel/service.go`:

```go
// ResolveRules looks up jurisdiction-scoped estoppel rules for the org's state.
// Returns an error if no rules are configured for the jurisdiction.
func (s *EstoppelService) ResolveRules(ctx context.Context, orgID, unitID uuid.UUID) (*EstoppelRules, error) {
	snap, err := s.property.GetPropertySnapshot(ctx, orgID, unitID)
	if err != nil {
		return nil, fmt.Errorf("resolving org state: %w", err)
	}
	if snap.OrgState == "" {
		return nil, api.NewUnprocessableError("organization has no state configured")
	}
	if s.jurisdictionRules == nil {
		return nil, api.NewUnprocessableError("jurisdiction rules not configured")
	}
	rules, err := s.jurisdictionRules.GetEstoppelRules(ctx, snap.OrgState)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		return nil, api.NewUnprocessableError(fmt.Sprintf("no estoppel rules configured for jurisdiction %q", snap.OrgState))
	}
	return rules, nil
}
```

Add `jurisdictionRules JurisdictionRulesRepository` to the service struct and constructor.

- [ ] **Step 2: Replace defaultEstoppelRules in handler**

In `handler.go`, update `CreateRequest`:

```go
func (h *Handler) CreateRequest(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	var dto CreateEstoppelRequestDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	rules, err := h.service.ResolveRules(r.Context(), orgID, dto.UnitID)
	if err != nil {
		h.logger.Error("ResolveRules failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	created, err := h.service.CreateRequest(r.Context(), orgID, dto, rules, userID)
	if err != nil {
		h.logger.Error("CreateRequest failed", "org_id", orgID, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}
```

Remove the `defaultEstoppelRules()` function.

- [ ] **Step 3: Update test fixtures to include jurisdiction rules mock**

In `service_test.go` and `handler_test.go`, update `newTestService()` / test server setup to pass a `mockJurisdictionRulesRepo` with Florida rules. Update `mockPropertyProvider` to return `OrgState: "FL"`.

- [ ] **Step 4: Run tests**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
```

- [ ] **Step 5: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ && git commit -m "feat(estoppel): replace hardcoded rules with jurisdiction_rules lookup

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Missing Endpoint — Update Narratives

**Files:**
- Modify: `backend/internal/estoppel/handler.go`
- Modify: `backend/internal/estoppel/handler_test.go`
- Modify: `backend/internal/estoppel/routes.go`
- Modify: `backend/internal/estoppel/service.go`

- [ ] **Step 1: Add UpdateNarratives service method**

Add to `service.go`:

```go
// UpdateNarratives stores manager-edited narrative sections on the request.
// Only allowed in manager_review status.
func (s *EstoppelService) UpdateNarratives(ctx context.Context, requestID uuid.UUID, dto UpdateNarrativesDTO, editedBy uuid.UUID) (*EstoppelRequest, error) {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	if req.Status != "manager_review" {
		return nil, api.NewValidationError("narratives can only be edited in manager_review status", "status")
	}

	narrativeJSON, err := json.Marshal(dto.Narratives)
	if err != nil {
		return nil, fmt.Errorf("marshalling narratives: %w", err)
	}

	updated, err := s.repo.UpdateRequestNarratives(ctx, requestID, narrativeJSON)
	if err != nil {
		return nil, fmt.Errorf("updating narratives: %w", err)
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        req.OrgID,
		ActorID:      editedBy,
		Action:       "estoppel_request.narratives_edited",
		ResourceType: "estoppel_request",
		ResourceID:   requestID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	return updated, nil
}
```

- [ ] **Step 2: Add handler**

Add to `handler.go`:

```go
// UpdateNarratives handles PATCH /organizations/{org_id}/estoppel/requests/{id}/narratives.
func (h *Handler) UpdateNarratives(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	var dto UpdateNarrativesDTO
	if err := api.ReadJSON(r, &dto); err != nil {
		api.WriteError(w, err)
		return
	}

	updated, err := h.service.UpdateNarratives(r.Context(), id, dto, userID)
	if err != nil {
		h.logger.Error("UpdateNarratives failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, updated)
}
```

- [ ] **Step 3: Add route**

Add to `routes.go`:

```go
mux.Handle("PATCH /api/v1/organizations/{org_id}/estoppel/requests/{id}/narratives",
	permMw("estoppel.request.approve", handler.UpdateNarratives))
```

- [ ] **Step 4: Run tests and commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ && git commit -m "feat(estoppel): add UpdateNarratives endpoint for manager review

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Missing Endpoint — Preview Certificate

**Files:**
- Modify: `backend/internal/estoppel/service.go`
- Modify: `backend/internal/estoppel/handler.go`
- Modify: `backend/internal/estoppel/routes.go`

- [ ] **Step 1: Add GeneratePreviewPDF service method**

Add to `service.go`:

```go
// GeneratePreviewPDF aggregates fresh data and generates a draft PDF without
// persisting anything. The returned bytes include a "DRAFT" watermark.
func (s *EstoppelService) GeneratePreviewPDF(ctx context.Context, requestID uuid.UUID, rules *EstoppelRules) ([]byte, error) {
	req, err := s.GetRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}

	data, err := s.AggregateData(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("aggregating data for preview: %w", err)
	}

	if req.RequestType == "lender_questionnaire" {
		return s.generator.GenerateLenderQuestionnaire(data, rules)
	}
	return s.generator.GenerateEstoppel(data, rules)
}
```

- [ ] **Step 2: Add handler**

Add to `handler.go`:

```go
// PreviewCertificate handles GET /organizations/{org_id}/estoppel/requests/{id}/preview.
// Returns PDF bytes directly with Content-Type: application/pdf.
func (h *Handler) PreviewCertificate(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	// Resolve rules for this org's jurisdiction.
	req, err := h.service.GetRequest(r.Context(), id)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	rules, err := h.service.ResolveRules(r.Context(), orgID, req.UnitID)
	if err != nil {
		h.logger.Error("ResolveRules for preview failed", "error", err)
		api.WriteError(w, err)
		return
	}

	pdfBytes, err := h.service.GeneratePreviewPDF(r.Context(), id, rules)
	if err != nil {
		h.logger.Error("PreviewCertificate failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=\"preview-%s.pdf\"", id))
	w.WriteHeader(http.StatusOK)
	w.Write(pdfBytes)
}
```

Add `"fmt"` to the handler imports if not present.

- [ ] **Step 3: Add route**

```go
mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/requests/{id}/preview",
	permMw("estoppel.request.approve", handler.PreviewCertificate))
```

- [ ] **Step 4: Run tests and commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ && git commit -m "feat(estoppel): add PreviewCertificate endpoint for draft PDF

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 8: Missing Endpoint — Download Certificate

**Files:**
- Modify: `backend/internal/estoppel/handler.go`
- Modify: `backend/internal/estoppel/routes.go`
- Modify: `backend/internal/estoppel/service.go`

- [ ] **Step 1: Add GetDownloadURL to service**

The service needs a way to get download URLs. Add a `DocumentDownloader` interface:

Add to `doc_adapter.go`:

```go
// DocumentDownloader retrieves pre-signed download URLs for stored documents.
type DocumentDownloader interface {
	GetDownloadURL(ctx context.Context, documentID uuid.UUID) (string, error)
}
```

Add `docDownloader DocumentDownloader` to the service struct, constructor, and all callers.

Add to `service.go`:

```go
// GetCertificateDownloadURL returns a pre-signed URL for downloading the
// certificate PDF.
func (s *EstoppelService) GetCertificateDownloadURL(ctx context.Context, certID uuid.UUID, downloadedBy uuid.UUID) (string, error) {
	cert, err := s.repo.FindCertificateByID(ctx, certID)
	if err != nil {
		return "", fmt.Errorf("finding certificate: %w", err)
	}
	if cert == nil {
		return "", api.NewNotFoundError("certificate not found")
	}
	if cert.DocumentID == nil {
		return "", api.NewUnprocessableError("certificate has no stored document")
	}

	url, err := s.docDownloader.GetDownloadURL(ctx, *cert.DocumentID)
	if err != nil {
		return "", fmt.Errorf("getting download URL: %w", err)
	}

	_ = s.auditor.Record(ctx, audit.AuditEntry{
		OrgID:        cert.OrgID,
		ActorID:      downloadedBy,
		Action:       "estoppel_certificate.downloaded",
		ResourceType: "estoppel_certificate",
		ResourceID:   certID,
		Module:       "estoppel",
		OccurredAt:   time.Now(),
	})

	return url, nil
}
```

- [ ] **Step 2: Add doc adapter implementation**

Add to `backend/internal/doc/estoppel_adapter.go`:

```go
// GetDownloadURL returns a pre-signed download URL for the given document.
func (a *EstoppelDocumentAdapter) GetDownloadURL(ctx context.Context, documentID uuid.UUID) (string, error) {
	return a.service.GetDownloadURL(ctx, documentID)
}
```

- [ ] **Step 3: Add handler**

```go
// DownloadCertificate handles GET /organizations/{org_id}/estoppel/certificates/{id}/download.
func (h *Handler) DownloadCertificate(w http.ResponseWriter, r *http.Request) {
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	url, err := h.service.GetCertificateDownloadURL(r.Context(), id, userID)
	if err != nil {
		h.logger.Error("DownloadCertificate failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}
```

- [ ] **Step 4: Add route**

```go
mux.Handle("GET /api/v1/organizations/{org_id}/estoppel/certificates/{id}/download",
	permMw("estoppel.certificate.download", handler.DownloadCertificate))
```

- [ ] **Step 5: Run tests and commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ backend/internal/doc/ && git commit -m "feat(estoppel): add DownloadCertificate endpoint with pre-signed URL

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 9: Missing Endpoint — Amend Certificate

**Files:**
- Modify: `backend/internal/estoppel/requests.go`
- Modify: `backend/internal/estoppel/handler.go`
- Modify: `backend/internal/estoppel/routes.go`
- Modify: `backend/internal/estoppel/service.go`

- [ ] **Step 1: Add AmendCertificateDTO**

Add to `requests.go`:

```go
// AmendCertificateDTO carries the fields for creating an amendment request.
type AmendCertificateDTO struct {
	Reason string `json:"reason,omitempty"`
}
```

- [ ] **Step 2: Add AmendCertificate service method**

```go
// AmendCertificate creates a new estoppel request that amends an existing
// certificate. The new request goes through the full workflow.
func (s *EstoppelService) AmendCertificate(ctx context.Context, certID uuid.UUID, rules *EstoppelRules, createdBy uuid.UUID) (*EstoppelRequest, error) {
	cert, err := s.repo.FindCertificateByID(ctx, certID)
	if err != nil {
		return nil, fmt.Errorf("finding certificate to amend: %w", err)
	}
	if cert == nil {
		return nil, api.NewNotFoundError("certificate not found")
	}

	// Look up the original request to copy fields.
	origReq, err := s.repo.FindRequestByID(ctx, cert.RequestID)
	if err != nil {
		return nil, err
	}

	// Determine fees: free if rules say so.
	rush := false
	if origReq != nil {
		rush = origReq.RushRequested
	}
	fees := CalculateFees(rules, rush, false)
	if rules.FreeAmendmentOnError {
		fees = FeeBreakdown{} // all zeros
	}
	deadline := CalculateDeadline(rules, rush, time.Now())

	amendReq := &EstoppelRequest{
		OrgID:           cert.OrgID,
		UnitID:          cert.UnitID,
		RequestType:     origReq.RequestType,
		RequestorType:   origReq.RequestorType,
		RequestorName:   origReq.RequestorName,
		RequestorEmail:  origReq.RequestorEmail,
		RequestorPhone:  origReq.RequestorPhone,
		RequestorCompany: origReq.RequestorCompany,
		PropertyAddress: origReq.PropertyAddress,
		OwnerName:       origReq.OwnerName,
		RushRequested:   rush,
		Status:          "submitted",
		FeeCents:        fees.FeeCents,
		TotalFeeCents:   fees.TotalFeeCents,
		DeadlineAt:      deadline,
		AmendmentOf:     &certID,
		Metadata:        map[string]any{},
		CreatedBy:       createdBy,
	}

	created, err := s.repo.CreateRequest(ctx, amendReq)
	if err != nil {
		return nil, fmt.Errorf("creating amendment request: %w", err)
	}

	_ = s.publisher.Publish(ctx, newEstoppelEvent(EventCertificateAmended, created.ID, cert.OrgID, map[string]any{
		"original_certificate_id": certID,
	}))

	return created, nil
}
```

- [ ] **Step 3: Add handler**

```go
// AmendCertificate handles POST /organizations/{org_id}/estoppel/certificates/{id}/amend.
func (h *Handler) AmendCertificate(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseEstoppelPathUUID(r, "org_id")
	if err != nil {
		api.WriteError(w, err)
		return
	}
	id, err := parseEstoppelPathUUID(r, "id")
	if err != nil {
		api.WriteError(w, err)
		return
	}

	userID := middleware.UserIDFromContext(r.Context())
	if userID == uuid.Nil {
		api.WriteError(w, api.NewUnauthenticatedError("user identity required"))
		return
	}

	// Resolve rules for fee calculation.
	cert, err := h.service.GetCertificate(r.Context(), id)
	if err != nil {
		api.WriteError(w, err)
		return
	}
	rules, err := h.service.ResolveRules(r.Context(), orgID, cert.UnitID)
	if err != nil {
		h.logger.Error("ResolveRules for amend failed", "error", err)
		api.WriteError(w, err)
		return
	}

	created, err := h.service.AmendCertificate(r.Context(), id, rules, userID)
	if err != nil {
		h.logger.Error("AmendCertificate failed", "id", id, "error", err)
		api.WriteError(w, err)
		return
	}

	api.WriteJSON(w, http.StatusCreated, created)
}
```

- [ ] **Step 4: Add GetCertificate service method if missing**

Check if `GetCertificate` exists on the service. If not, add:

```go
func (s *EstoppelService) GetCertificate(ctx context.Context, id uuid.UUID) (*EstoppelCertificate, error) {
	cert, err := s.repo.FindCertificateByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding certificate: %w", err)
	}
	if cert == nil {
		return nil, api.NewNotFoundError("certificate not found")
	}
	return cert, nil
}
```

- [ ] **Step 5: Add route**

```go
mux.Handle("POST /api/v1/organizations/{org_id}/estoppel/certificates/{id}/amend",
	permMw("estoppel.request.approve", handler.AmendCertificate))
```

- [ ] **Step 6: Run tests and commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ && git commit -m "feat(estoppel): add AmendCertificate endpoint for corrections

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 10: State-Specific Templates

**Files:**
- Modify: `backend/internal/estoppel/templates.go`
- Modify: `backend/internal/estoppel/generator.go`
- Modify: `backend/internal/estoppel/generator_test.go`

- [ ] **Step 1: Write tests for state-specific templates**

Add to `generator_test.go`:

```go
func TestMarotoGenerator_FloridaTemplate(t *testing.T) {
	gen := NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &EstoppelRules{StatutoryFormID: "fl_720_30851", StatuteRef: "§720.30851"}
	pdf, err := gen.GenerateEstoppel(data, rules)
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(pdf[:4]))
}

func TestMarotoGenerator_CaliforniaTemplate(t *testing.T) {
	gen := NewMarotoGenerator()
	data := newTestAggregatedData()
	rules := &EstoppelRules{StatutoryFormID: "ca_4528", StatuteRef: "Civil Code §4525–4530",
		RequiredAttachments: []string{"ccrs", "bylaws", "budget"}}
	pdf, err := gen.GenerateEstoppel(data, rules)
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(pdf[:4]))
}

func TestMarotoGenerator_TexasTemplate(t *testing.T) {
	gen := NewMarotoGenerator()
	data := newTestAggregatedData()
	effectiveDays := 60
	rules := &EstoppelRules{StatutoryFormID: "tx_207", StatuteRef: "Property Code §207.003", EffectivePeriodDays: &effectiveDays}
	pdf, err := gen.GenerateEstoppel(data, rules)
	require.NoError(t, err)
	assert.Equal(t, "%PDF", string(pdf[:4]))
}
```

- [ ] **Step 2: Add template selection to generator**

Update `GenerateEstoppel` in `generator.go` to select template by `rules.StatutoryFormID`:

```go
func (g *MarotoGenerator) GenerateEstoppel(data *AggregatedData, rules *EstoppelRules) ([]byte, error) {
	formID := rules.StatutoryFormID
	if formID == "" {
		formID = "generic"
	}
	builder, ok := templateBuilders[formID]
	if !ok {
		builder = buildEstoppelPDF // fallback to generic
	}
	return builder(data, rules)
}
```

Add to `templates.go`:

```go
// templateBuilders maps statutory form IDs to template builder functions.
var templateBuilders = map[string]func(*AggregatedData, *EstoppelRules) ([]byte, error){
	"generic":      buildEstoppelPDF,
	"fl_720_30851": buildFloridaEstoppelPDF,
	"ca_4528":      buildCaliforniaEstoppelPDF,
	"tx_207":       buildTexasEstoppelPDF,
	"nv_116":       buildNevadaEstoppelPDF,
	"va_55_1":      buildVirginiaEstoppelPDF,
}
```

- [ ] **Step 3: Implement 5 state-specific builders**

Each builder reuses `newMarotoDoc()` and the helper functions but adds state-specific sections. Add these to `templates.go`:

**Florida** (`buildFloridaEstoppelPDF`): Starts with "FLORIDA ESTOPPEL CERTIFICATE — §720.30851" header, adds all standard sections, then adds a "STATUTORY QUESTIONS" section with 19 numbered items covering the required disclosure points.

**California** (`buildCaliforniaEstoppelPDF`): Starts with "CALIFORNIA COMMON INTEREST DEVELOPMENT — §4528" header, adds standard sections, then adds a "REQUIRED DOCUMENT ATTACHMENTS" section listing each item in `rules.RequiredAttachments`.

**Texas** (`buildTexasEstoppelPDF`): Starts with standard header, adds a bold "VALIDITY NOTICE: This certificate is valid for 60 days from the date of issuance per Texas Property Code §207.003" after the header, and adds a "DAMAGES DISCLOSURE" section noting up to $5,000 damages for non-compliance.

**Nevada** (`buildNevadaEstoppelPDF`): Starts with standard header, adds "FEE NOTICE: Fees subject to CPI adjustment (3% annual cap) per NRS 116.4109" and "ELECTRONIC FORMAT: This certificate is provided in electronic format as required by Nevada law."

**Virginia** (`buildVirginiaEstoppelPDF`): Starts with standard header, adds "BUYER RESCISSION: Buyer has a 3-day rescission period from receipt of this certificate per §55.1-1808" and "CIC BOARD REGISTRATION" section.

Each reuses `buildEstoppelPDF` internally for the common sections, then adds state-specific sections before generating.

- [ ] **Step 4: Run tests**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./internal/estoppel/ -v -count=1 -run "TestMaroto"
```

- [ ] **Step 5: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/internal/estoppel/ && git commit -m "feat(estoppel): add state-specific PDF templates for FL, CA, TX, NV, VA

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

### Task 11: Wire Phase 2 into main.go and Verify

**Files:**
- Modify: `backend/cmd/quorant-api/main.go`

- [ ] **Step 1: Update main.go wiring**

Update the estoppel module section in `main.go` to pass the new dependencies:

```go
// --- Estoppel module ---
estoppelRepo := estoppel.NewPostgresRepository(pool)
jurisdictionRulesRepo := estoppel.NewPostgresJurisdictionRulesRepository(pool)
financialProvider := fin.NewEstoppelFinancialAdapter(finService)
complianceProvider := gov.NewEstoppelComplianceAdapter(govService)
propertyProvider := org.NewEstoppelPropertyAdapter(orgService)
narrativeGen := estoppel.NewNoopNarrativeGenerator()
pdfGen := estoppel.NewMarotoGenerator()
docAdapter := doc.NewEstoppelDocumentAdapter(docService)
estoppelService := estoppel.NewEstoppelService(
	estoppelRepo,
	financialProvider,
	complianceProvider,
	propertyProvider,
	narrativeGen,
	pdfGen,
	docAdapter,        // DocumentUploader
	docAdapter,        // DocumentDownloader
	jurisdictionRulesRepo,
	auditor,
	outboxPublisher,
	logger,
)
estoppelHandler := estoppel.NewHandler(estoppelService, logger)
estoppel.RegisterRoutes(mux, estoppelHandler, tokenValidator, permChecker, entitlementChecker, resolveUserID)
```

- [ ] **Step 2: Full build verification**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go build ./cmd/quorant-api/
```

- [ ] **Step 3: Full test suite**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go test ./... -count=1 -short 2>&1 | tail -30
```

- [ ] **Step 4: Vet**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel/backend && go vet ./internal/estoppel/...
```

- [ ] **Step 5: Commit**

```bash
cd /home/douglasl/Projects/quorant/.worktrees/estoppel && git add backend/cmd/quorant-api/main.go && git commit -m "feat(estoppel): wire Phase 2 dependencies into main.go

Co-Authored-By: Claude Opus 4.6 (1M context) <noreply@anthropic.com>"
```

---

## Verification

After all tasks complete:

1. `go build ./cmd/quorant-api/` — binary compiles
2. `go test ./... -count=1 -short` — all tests pass, zero regressions
3. `go vet ./internal/estoppel/...` — no issues
4. `git log --oneline feature/estoppel-module ^main` — all commits present
5. 9 routes registered: verify with `grep -c "mux.Handle" backend/internal/estoppel/routes.go` = 9
6. No `defaultEstoppelRules` — verify with `grep defaultEstoppelRules backend/internal/estoppel/handler.go` returns nothing
