# Estoppel Module Phase 2 — Design Spec

**Date:** 2026-04-09
**Parent:** `docs/superpowers/specs/2026-04-09-estoppel-certificate-generation-design.md`
**Status:** Draft
**Branch:** `feature/estoppel-module`

---

## Context

Phase 1 delivered the estoppel module foundation: domain types, data providers, repository, PDF generation (Maroto v2), service orchestration, and 5 HTTP endpoints. Phase 2 completes the workflow by wiring document storage, adding the remaining endpoints, integrating jurisdiction-scoped compliance rules, and differentiating state-specific PDF templates.

---

## Enhancement 1: Document Storage Integration

### Problem
`GenerateCertificate` produces PDF bytes but discards them. `DocumentID` on `EstoppelCertificate` is nullable and always null. Certificates cannot be downloaded.

### Solution
Extend the `doc` module with a server-side upload method, then wire it into the estoppel service.

**New method on DocService** (`backend/internal/doc/service.go`):
```go
type UploadFromBytesRequest struct {
    Title       string
    FileName    string
    ContentType string
    SizeBytes   int64
    Visibility  string
    CategoryID  *uuid.UUID
}

func (s *DocService) UploadFromBytes(
    ctx context.Context,
    orgID uuid.UUID,
    req UploadFromBytesRequest,
    data []byte,
    uploadedBy uuid.UUID,
) (*Document, error)
```

Internally: generates a storage key (`estoppel/{orgID}/{certID}.pdf`), calls `storage.Upload()` to put bytes into S3, creates a Document record via the repository, returns the Document with its ID.

**EstoppelService changes:**
- Add `docService` field (type: interface `DocumentUploader` with `UploadFromBytes` method, defined in estoppel package)
- `GenerateCertificate` calls `docService.UploadFromBytes()` after generating PDF, sets `DocumentID` on the certificate before persisting

**DocumentUploader interface** (in estoppel package, to avoid circular import):
```go
type DocumentUploader interface {
    UploadFromBytes(ctx context.Context, orgID uuid.UUID, req UploadFromBytesRequest, data []byte, uploadedBy uuid.UUID) (documentID uuid.UUID, err error)
}
```

An adapter in the doc package implements this interface wrapping DocService.

---

## Enhancement 2: Missing Endpoints

### PATCH /api/v1/organizations/{org_id}/estoppel/requests/{id}/narratives
- **Permission:** `estoppel.request.approve`
- **Body:** `UpdateNarrativesDTO` (already defined in `requests.go`)
- **Logic:** Validates request is in `manager_review` status. Stores updated narrative sections in request metadata via `repo.UpdateRequestNarratives()`. Returns updated request.
- **Test:** Handler test verifying 200 on valid update, 400 on invalid status.

### GET /api/v1/organizations/{org_id}/estoppel/requests/{id}/preview
- **Permission:** `estoppel.request.approve`
- **Logic:** Calls `service.AggregateData()` to get fresh data, then calls `service.GeneratePreviewPDF()` (a new service method). Returns PDF bytes directly with `Content-Type: application/pdf`. Adds "DRAFT" watermark text to the first page via a Maroto row.
- **Does NOT persist** the PDF or create a certificate record.
- **Test:** Handler test verifying 200 with `application/pdf` content type and `%PDF` magic bytes.

### GET /api/v1/organizations/{org_id}/estoppel/certificates/{id}/download
- **Permission:** `estoppel.certificate.download`
- **Logic:** Finds certificate, gets its `DocumentID`, calls `DocService.GetDownloadURL()` to get a pre-signed S3 GET URL (15-min expiry). Returns JSON `{"url": "https://..."}`.
- **Error:** 404 if certificate not found, 422 if `DocumentID` is null (document not yet stored).
- **Audit:** Records `estoppel.certificate.downloaded` audit entry.
- **Test:** Handler test with mock doc service returning a URL.

### POST /api/v1/organizations/{org_id}/estoppel/certificates/{id}/amend
- **Permission:** `estoppel.request.approve`
- **Body:** `AmendCertificateDTO` — optional reason field
- **Logic:** Finds original certificate, creates a new `EstoppelRequest` with `amendment_of = original_certificate_id`. Request goes through the full workflow (submitted → ... → delivered). Fee calculation: if `rules.FreeAmendmentOnError` is true, total fee is 0.
- **Test:** Handler test verifying new request is created with correct `amendment_of`.

---

## Enhancement 3: PolicyResolver Integration

### Problem
`CreateRequest` handler uses `defaultEstoppelRules()` with hardcoded $299 fee. Jurisdiction-scoped rules exist in `jurisdiction_rules` table but are never queried.

### Solution

**New: JurisdictionRulesRepository** (in estoppel package):
```go
type JurisdictionRulesRepository interface {
    GetEstoppelRules(ctx context.Context, jurisdiction string) (*EstoppelRules, error)
}
```

**PostgreSQL implementation:** Queries `jurisdiction_rules` where `rule_category = 'estoppel'` AND `rule_key = 'estoppel_rules'` AND `jurisdiction = $1` AND `(expiration_date IS NULL OR expiration_date > now())`. Deserializes `value` JSONB into `EstoppelRules`.

Note: `jurisdiction_rules` has NO RLS (platform-managed table), so no tenant context needed.

**Handler changes:**
- `CreateRequest` handler:
  1. Resolves org's state via `service.property.GetPropertySnapshot()` (already available)
  2. Calls `jurisdictionRepo.GetEstoppelRules(ctx, orgState)`
  3. If not found, returns `422 Unprocessable: no estoppel rules configured for jurisdiction {state}`
  4. Passes rules to `service.CreateRequest()`
- Remove `defaultEstoppelRules()` function

**Service changes:**
- Add `jurisdictionRules JurisdictionRulesRepository` to service struct
- Add `ResolveRules(ctx, orgID) (*EstoppelRules, error)` method that gets the org state and looks up rules

---

## Enhancement 4: State-Specific Templates

### Problem
All states use the same generic template. The spec requires state-specific variations for statutory compliance.

### Solution

Expand the template registry in `templates.go` with 5 state-specific builders:

| Form ID | State | Key Differences |
|---------|-------|----------------|
| `fl_720_30851` | Florida | 19 statutory questions in prescribed order per §720.30851 |
| `ca_4528` | California | §4528 standardized form + "Required Documents" attachment list section |
| `tx_207` | Texas | "VALIDITY NOTICE: This certificate is valid for 60 days" + §207.003 damages disclosure |
| `nv_116` | Nevada | "Fee subject to CPI adjustment (3% cap)" notice + electronic format declaration |
| `va_55_1` | Virginia | "BUYER RESCISSION: 3-day rescission period applies" + CIC Board registration number field |

**Template selection flow:**
1. `rules.StatutoryFormID` is set from jurisdiction rules (e.g., `"fl_720_30851"`)
2. `generator.GenerateEstoppel()` looks up the builder in `templateRegistry`
3. If found, uses the state-specific builder
4. If not found, falls back to `buildGenericEstoppelTemplate`

Each state-specific builder reuses the common helpers (`addSectionHeader`, `addLabelValue`, etc.) but arranges sections differently and adds state-specific disclosures.

---

## Testing Strategy

| Enhancement | Test Approach |
|------------|---------------|
| Document storage | Unit test: mock DocumentUploader, verify GenerateCertificate calls it and sets DocumentID |
| Missing endpoints | Handler tests via httptest: verify status codes, response bodies, content types |
| PolicyResolver | Unit test: mock JurisdictionRulesRepository, verify correct rules returned per state |
| State templates | Generator tests: verify each state template produces valid PDF with state-specific text |

---

## Files Changed

| File | Change |
|------|--------|
| `doc/service.go` | Add `UploadFromBytes` method |
| `doc/service_test.go` | Test for `UploadFromBytes` |
| `estoppel/service.go` | Wire DocumentUploader, add ResolveRules, add GeneratePreviewPDF |
| `estoppel/service_test.go` | Tests for new service methods |
| `estoppel/handler.go` | 4 new handlers + remove defaultEstoppelRules |
| `estoppel/handler_test.go` | Tests for new handlers |
| `estoppel/routes.go` | 4 new route registrations |
| `estoppel/requests.go` | AmendCertificateDTO |
| `estoppel/templates.go` | 5 state-specific template builders |
| `estoppel/generator_test.go` | Tests for state-specific templates |
| `estoppel/jurisdiction_rules.go` | JurisdictionRulesRepository interface + Postgres impl |
| `estoppel/jurisdiction_rules_test.go` | Integration tests |
| `cmd/quorant-api/main.go` | Wire new dependencies |
