# Fin Module: Enum Constants, Payment Idempotency, Compliance Wiring

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close issues #79, #80, and #84 — type-safe enum constants across the fin module, idempotency keys on payment recording, and wiring the ComplianceResolver into late fees, fund transfers, and collection actions.

**Architecture:** Three independent workstreams that share the `fin` module. Issue #79 (enums) should land first because it introduces typed constants used by the other two. Issue #80 adds an `idempotency_key` column/field to payments with a unique constraint and ON CONFLICT deduplication. Issue #84 calls `ComplianceResolver.CheckCompliance()` and `GetJurisdictionRule()` at three business-rule enforcement points.

**Tech Stack:** Go 1.22, pgx v5, testify, Atlas migrations, PostgreSQL 16

---

## File Map

| Action | File | Responsibility |
|--------|------|---------------|
| Modify | `backend/internal/fin/enums.go` | Add `FundTransactionType` and `FundTransactionRefType` typed enums; add `FundTxTypeRevenue`/`FundTxTypeExpense` constants |
| Modify | `backend/internal/fin/engine_effects.go:21-26` | Change `FundTransactionDirective.Type` from `string` to `FundTransactionType` |
| Modify | `backend/internal/fin/engine_types.go:193-201` | Change `GLAccountSeed.Type` from `string` to `GLAccountType` |
| Modify | `backend/internal/fin/engine_ifrs.go` | Replace bare `"revenue"`/`"expense"` strings with typed constants |
| Modify | `backend/internal/fin/engine_gaap.go` | Replace bare `"revenue"`/`"expense"` strings with typed constants |
| Modify | `backend/internal/fin/domain.go:218-231` | Change `FundTransaction.TransactionType` from `string` to `FundTransactionType`; change `ReferenceType` from `*string` to `*FundTransactionRefType` |
| Modify | `backend/internal/fin/service.go:119-134` | Update `executeEffects` to use typed `FundTransactionType`/`FundTransactionRefType` |
| Modify | `backend/internal/fin/payment_postgres.go:163` | Replace SQL `'void'` literal with `PaymentStatusVoid` constant |
| Modify | `backend/internal/fin/domain.go:90-106` | Add `IdempotencyKey *string` field to `Payment` struct |
| Modify | `backend/internal/fin/requests.go:79-96` | Add `IdempotencyKey *string` field to `CreatePaymentRequest` |
| Modify | `backend/internal/fin/payment_repository.go` | Add `FindPaymentByIdempotencyKey` method to interface |
| Modify | `backend/internal/fin/payment_postgres.go` | Add `idempotency_key` to INSERT, add `FindPaymentByIdempotencyKey`, update `scanPayment` |
| Modify | `backend/internal/fin/service.go:590-722` | Add idempotency dedup check before creating payment |
| Create | `backend/migrations/20260411000001_payment_idempotency_key.sql` | Add `idempotency_key` column with partial unique index |
| Modify | `backend/internal/fin/service.go:300-330` | Call `ComplianceResolver.CheckCompliance` to cap late fees |
| Modify | `backend/internal/fin/service.go:1278-1373` | Call `ComplianceResolver.CheckCompliance` for reserve fund withdrawals |
| Modify | `backend/internal/fin/service.go:1405-1420` | Call `ComplianceResolver.CheckCompliance` before collection actions |
| Modify | `backend/internal/fin/service_test.go` | Add tests for idempotency and compliance wiring |

---

## Part 1: Domain Enum Constants (Issue #79)

### Task 1: Add typed FundTransactionType and FundTransactionRefType enums

**Files:**
- Modify: `backend/internal/fin/enums.go:136-147`

- [ ] **Step 1: Write the failing test**

Create `backend/internal/fin/enums_test.go` (or add to it if it exists):

```go
func TestFundTransactionType_IsValid(t *testing.T) {
	tests := []struct {
		val  fin.FundTransactionType
		want bool
	}{
		{fin.FundTxTypeTransferOut, true},
		{fin.FundTxTypeTransferIn, true},
		{fin.FundTxTypeLoanOut, true},
		{fin.FundTxTypeLoanIn, true},
		{fin.FundTxTypeRevenue, true},
		{fin.FundTxTypeExpense, true},
		{fin.FundTransactionType("bogus"), false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.val.IsValid(), "FundTransactionType(%q).IsValid()", tt.val)
	}
}

func TestFundTransactionRefType_IsValid(t *testing.T) {
	tests := []struct {
		val  fin.FundTransactionRefType
		want bool
	}{
		{fin.FundTxRefTypeTransfer, true},
		{fin.FundTransactionRefType("bogus"), false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.val.IsValid(), "FundTransactionRefType(%q).IsValid()", tt.val)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestFundTransactionType_IsValid -short -count=1`
Expected: compilation error — `FundTransactionType` type does not exist, `FundTxTypeRevenue` undefined.

- [ ] **Step 3: Implement the typed enums**

In `backend/internal/fin/enums.go`, replace lines 136-147:

```go
// FundTransactionType classifies the type of fund transaction.
type FundTransactionType string

const (
	FundTxTypeTransferOut FundTransactionType = "transfer_out"
	FundTxTypeTransferIn  FundTransactionType = "transfer_in"
	FundTxTypeLoanOut     FundTransactionType = "loan_out"
	FundTxTypeLoanIn      FundTransactionType = "loan_in"
	FundTxTypeRevenue     FundTransactionType = "revenue"
	FundTxTypeExpense     FundTransactionType = "expense"
)

// IsValid returns true if the FundTransactionType value is one of the defined constants.
func (s FundTransactionType) IsValid() bool {
	switch s {
	case FundTxTypeTransferOut, FundTxTypeTransferIn, FundTxTypeLoanOut, FundTxTypeLoanIn,
		FundTxTypeRevenue, FundTxTypeExpense:
		return true
	}
	return false
}

// FundTransactionRefType identifies the source entity for a fund transaction.
type FundTransactionRefType string

const (
	FundTxRefTypeTransfer FundTransactionRefType = "fund_transfer"
)

// IsValid returns true if the FundTransactionRefType value is one of the defined constants.
func (s FundTransactionRefType) IsValid() bool {
	switch s {
	case FundTxRefTypeTransfer:
		return true
	}
	return false
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run "TestFundTransaction(Type|RefType)_IsValid" -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/enums.go backend/internal/fin/enums_test.go
git commit -m "feat(fin): add typed FundTransactionType and FundTransactionRefType enums (issue #79)"
```

### Task 2: Update structs to use typed FundTransactionType

**Files:**
- Modify: `backend/internal/fin/engine_effects.go:21-26`
- Modify: `backend/internal/fin/domain.go:218-231`
- Modify: `backend/internal/fin/service.go:119-134`

- [ ] **Step 1: Change `FundTransactionDirective.Type` to `FundTransactionType`**

In `backend/internal/fin/engine_effects.go`, change line 23 from:

```go
	Type        string // uses FundTxType* constants from enums.go
```

to:

```go
	Type        FundTransactionType
```

- [ ] **Step 2: Change `FundTransaction.TransactionType` to `FundTransactionType` and `ReferenceType` to `*FundTransactionRefType`**

In `backend/internal/fin/domain.go`, change the `FundTransaction` struct:

```go
type FundTransaction struct {
	ID                uuid.UUID              `json:"id"`
	FundID            uuid.UUID              `json:"fund_id"`
	OrgID             uuid.UUID              `json:"org_id"`
	CurrencyCode      string                 `json:"currency_code"`
	TransactionType   FundTransactionType    `json:"transaction_type"`
	AmountCents       int64                  `json:"amount_cents"`
	BalanceAfterCents int64                  `json:"balance_after_cents"`
	Description       *string                `json:"description,omitempty"`
	ReferenceType     *FundTransactionRefType `json:"reference_type,omitempty"`
	ReferenceID       *uuid.UUID             `json:"reference_id,omitempty"`
	EffectiveDate     time.Time              `json:"effective_date"`
	CreatedAt         time.Time              `json:"created_at"`
}
```

- [ ] **Step 3: Update `executeEffects` in service.go to use typed ref**

In `backend/internal/fin/service.go`, around line 133, change:

```go
			refType := FundTxRefTypeTransfer
			txn.ReferenceType = &refType
```

This already compiles because `FundTxRefTypeTransfer` is now a `FundTransactionRefType` and `ReferenceType` is `*FundTransactionRefType`. No code change needed here — just verify it compiles.

- [ ] **Step 4: Build to verify compilation**

Run: `cd backend && go build ./...`
Expected: success (or compilation errors in engine files that still use bare strings — those are fixed in Task 3).

If there are compilation errors in `engine_ifrs.go` or `engine_gaap.go` because they assign `"revenue"` to a `FundTransactionType` field, that's expected — Task 3 fixes those.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/engine_effects.go backend/internal/fin/domain.go backend/internal/fin/service.go
git commit -m "refactor(fin): use typed FundTransactionType in structs (issue #79)"
```

### Task 3: Replace bare strings in engine files and chart of accounts

**Files:**
- Modify: `backend/internal/fin/engine_ifrs.go`
- Modify: `backend/internal/fin/engine_gaap.go`
- Modify: `backend/internal/fin/engine_types.go:193-201`

- [ ] **Step 1: Change `GLAccountSeed.Type` to `GLAccountType`**

In `backend/internal/fin/engine_types.go`, change the struct:

```go
type GLAccountSeed struct {
	Number    int
	ParentNum int
	Name      string
	Type      GLAccountType
	IsHeader  bool
	IsSystem  bool
	FundKey   string // "operating", "reserve", "capital", "special", or ""
}
```

- [ ] **Step 2: Update `engine_ifrs.go` — FundTransactionDirective Type fields**

Replace bare strings in fund transaction directives:

Line 272: `Type: "revenue"` → `Type: FundTxTypeRevenue`
Line 564: `Type: "expense"` → `Type: FundTxTypeExpense`

- [ ] **Step 3: Update `engine_ifrs.go` — year-end close switch statements**

Lines 747, 754, 761 — the switch cases compare `acctType` which is extracted from a `map[string]any`. These remain string comparisons because they come from a runtime map, but cast to `GLAccountType` for clarity:

```go
switch GLAccountType(acctType) {
case GLAccountTypeRevenue:
    // ... existing body ...
case GLAccountTypeExpense:
    // ... existing body ...
case GLAccountTypeEquity:
    // ... existing body ...
}
```

- [ ] **Step 4: Update `engine_ifrs.go` — chart of accounts seed data**

Replace all bare `Type: "asset"` etc. with typed constants in the `ifrsChartOfAccounts` var (lines 940-1005):

- `Type: "asset"` → `Type: GLAccountTypeAsset`
- `Type: "liability"` → `Type: GLAccountTypeLiability`
- `Type: "equity"` → `Type: GLAccountTypeEquity`
- `Type: "revenue"` → `Type: GLAccountTypeRevenue`
- `Type: "expense"` → `Type: GLAccountTypeExpense`

- [ ] **Step 5: Apply identical changes to `engine_gaap.go`**

Same replacements:
- Line 222: `Type: "revenue"` → `Type: FundTxTypeRevenue`
- Line 569: `Type: "expense"` → `Type: FundTxTypeExpense`
- Chart of accounts seed data: same `GLAccountType*` constant replacements
- Year-end close switch: same `GLAccountType()` cast pattern

- [ ] **Step 6: Build and test**

Run: `cd backend && go build ./... && go test ./internal/fin/... -short -count=1`
Expected: all compile, all tests pass.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/fin/engine_types.go backend/internal/fin/engine_ifrs.go backend/internal/fin/engine_gaap.go
git commit -m "refactor(fin): replace bare strings with typed enum constants in engines (issue #79)"
```

### Task 4: Replace SQL `'void'` literal in payment_postgres.go

**Files:**
- Modify: `backend/internal/fin/payment_postgres.go:160-173`

- [ ] **Step 1: Replace the SQL literal**

In `backend/internal/fin/payment_postgres.go`, change `UpdatePaymentVoid`:

```go
func (r *PostgresPaymentRepository) UpdatePaymentVoid(ctx context.Context, id uuid.UUID, voidedBy *uuid.UUID, voidedAt *time.Time) error {
	const q = `
		UPDATE payments
		SET status     = $1,
		    voided_by  = $2,
		    voided_at  = $3,
		    updated_at = now()
		WHERE id = $4`

	_, err := r.db.Exec(ctx, q, PaymentStatusVoid, voidedBy, voidedAt, id)
	if err != nil {
		return fmt.Errorf("fin: UpdatePaymentVoid: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Build to verify**

Run: `cd backend && go build ./...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/payment_postgres.go
git commit -m "refactor(fin): use PaymentStatusVoid constant in SQL (issue #79)"
```

---

## Part 2: Payment Idempotency Keys (Issue #80)

### Task 5: Add migration for idempotency_key column

**Files:**
- Create: `backend/migrations/20260411000001_payment_idempotency_key.sql`

- [ ] **Step 1: Write the migration**

```sql
-- Add idempotency key to payments for duplicate submission protection.
ALTER TABLE payments ADD COLUMN idempotency_key TEXT;

-- Partial unique index: only enforce uniqueness on non-null keys.
-- This allows legacy rows and payments without idempotency keys to coexist.
CREATE UNIQUE INDEX idx_payments_idempotency_key
    ON payments (org_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;
```

- [ ] **Step 2: Commit**

```bash
git add backend/migrations/20260411000001_payment_idempotency_key.sql
git commit -m "feat(fin): add idempotency_key column to payments table (issue #80)"
```

### Task 6: Add IdempotencyKey to domain struct, request, and repository

**Files:**
- Modify: `backend/internal/fin/domain.go:90-106`
- Modify: `backend/internal/fin/requests.go:79-96`
- Modify: `backend/internal/fin/payment_repository.go`
- Modify: `backend/internal/fin/payment_postgres.go`

- [ ] **Step 1: Write the failing test**

In `backend/internal/fin/service_test.go`:

```go
func TestRecordPayment_IdempotencyKey_DeduplicatesPayment(t *testing.T) {
	svc, _, paymentRepo, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()
	key := "txn-abc-123"

	req := fin.CreatePaymentRequest{
		UnitID:         uuid.New(),
		AmountCents:    10000,
		IdempotencyKey: &key,
	}

	first, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)

	second, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)

	assert.Equal(t, first.ID, second.ID, "second call should return the original payment")
	assert.Len(t, paymentRepo.payments, 1, "only one payment should exist")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestRecordPayment_IdempotencyKey_DeduplicatesPayment -short -count=1`
Expected: compilation error — `IdempotencyKey` field does not exist on `CreatePaymentRequest`.

- [ ] **Step 3: Add `IdempotencyKey` to `Payment` domain struct**

In `backend/internal/fin/domain.go`, add the field to the `Payment` struct after `Description`:

```go
	Description    *string       `json:"description,omitempty"`
	IdempotencyKey *string       `json:"idempotency_key,omitempty"`
```

- [ ] **Step 4: Add `IdempotencyKey` to `CreatePaymentRequest`**

In `backend/internal/fin/requests.go`, add the field:

```go
type CreatePaymentRequest struct {
	UnitID          uuid.UUID  `json:"unit_id"`
	AmountCents     int64      `json:"amount_cents"`
	PaymentMethodID *uuid.UUID `json:"payment_method_id,omitempty"`
	Description     *string    `json:"description,omitempty"`
	IdempotencyKey  *string    `json:"idempotency_key,omitempty"`
}
```

- [ ] **Step 5: Add `FindPaymentByIdempotencyKey` to `PaymentRepository` interface**

In `backend/internal/fin/payment_repository.go`, add after `FindPaymentByID`:

```go
	// FindPaymentByIdempotencyKey returns the payment matching the given
	// org-scoped idempotency key, or nil, nil if no match exists.
	FindPaymentByIdempotencyKey(ctx context.Context, orgID uuid.UUID, key string) (*Payment, error)
```

- [ ] **Step 6: Update `payment_postgres.go` — add `idempotency_key` to INSERT and scan**

Update `CreatePayment` SQL to include `idempotency_key`:

```go
func (r *PostgresPaymentRepository) CreatePayment(ctx context.Context, p *Payment) (*Payment, error) {
	const q = `
		INSERT INTO payments (
			org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
			status, provider_ref, description, idempotency_key, paid_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		RETURNING id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		          status, provider_ref, description, idempotency_key, paid_at,
		          voided_by, voided_at, created_at, updated_at`

	row := r.db.QueryRow(ctx, q,
		p.OrgID,
		p.CurrencyCode,
		p.UnitID,
		p.UserID,
		p.PaymentMethodID,
		p.AmountCents,
		p.Status,
		p.ProviderRef,
		p.Description,
		p.IdempotencyKey,
		p.PaidAt,
	)

	result, err := scanPayment(row)
	if err != nil {
		return nil, fmt.Errorf("fin: CreatePayment: %w", err)
	}
	return result, nil
}
```

Update `FindPaymentByID`, `ListPaymentsByOrg`, `ListPaymentsByUnit` SELECT columns to include `idempotency_key` (add it after `description` in each SELECT list).

Add `FindPaymentByIdempotencyKey`:

```go
func (r *PostgresPaymentRepository) FindPaymentByIdempotencyKey(ctx context.Context, orgID uuid.UUID, key string) (*Payment, error) {
	const q = `
		SELECT id, org_id, currency_code, unit_id, user_id, payment_method_id, amount_cents,
		       status, provider_ref, description, idempotency_key, paid_at,
		       voided_by, voided_at, created_at, updated_at
		FROM payments
		WHERE org_id = $1 AND idempotency_key = $2`

	row := r.db.QueryRow(ctx, q, orgID, key)
	result, err := scanPayment(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fin: FindPaymentByIdempotencyKey: %w", err)
	}
	return result, nil
}
```

Update `scanPayment` to include `IdempotencyKey`:

```go
func scanPayment(row pgx.Row) (*Payment, error) {
	var p Payment
	err := row.Scan(
		&p.ID,
		&p.OrgID,
		&p.CurrencyCode,
		&p.UnitID,
		&p.UserID,
		&p.PaymentMethodID,
		&p.AmountCents,
		&p.Status,
		&p.ProviderRef,
		&p.Description,
		&p.IdempotencyKey,
		&p.PaidAt,
		&p.VoidedBy,
		&p.VoidedAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}
```

- [ ] **Step 7: Add `FindPaymentByIdempotencyKey` to mock in `service_test.go`**

```go
func (m *mockPaymentRepo) FindPaymentByIdempotencyKey(_ context.Context, orgID uuid.UUID, key string) (*fin.Payment, error) {
	for i := range m.payments {
		if m.payments[i].OrgID == orgID && m.payments[i].IdempotencyKey != nil && *m.payments[i].IdempotencyKey == key {
			out := m.payments[i]
			return &out, nil
		}
	}
	return nil, nil
}
```

- [ ] **Step 8: Wire idempotency check in `RecordPayment` service method**

In `backend/internal/fin/service.go`, at the top of `RecordPayment` (after validation, before building the `Payment` struct), add:

```go
	// Idempotency: if the caller supplied a key, check for an existing payment.
	if req.IdempotencyKey != nil {
		existing, findErr := s.payments.FindPaymentByIdempotencyKey(ctx, orgID, *req.IdempotencyKey)
		if findErr != nil {
			return nil, fmt.Errorf("fin: RecordPayment idempotency lookup: %w", findErr)
		}
		if existing != nil {
			return existing, nil
		}
	}
```

And when building the `Payment` struct, set the key:

```go
	p := &Payment{
		OrgID:           orgID,
		CurrencyCode:    "USD",
		UnitID:          req.UnitID,
		UserID:          userID,
		PaymentMethodID: req.PaymentMethodID,
		AmountCents:     req.AmountCents,
		Status:          PaymentStatusCompleted,
		Description:     req.Description,
		IdempotencyKey:  req.IdempotencyKey,
		PaidAt:          &now,
	}
```

- [ ] **Step 9: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestRecordPayment -short -count=1`
Expected: all RecordPayment tests pass, including the new idempotency test.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/fin/domain.go backend/internal/fin/requests.go \
       backend/internal/fin/payment_repository.go backend/internal/fin/payment_postgres.go \
       backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): add idempotency key to payment recording (issue #80)"
```

### Task 7: Test that nil idempotency key skips dedup

**Files:**
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write the test**

```go
func TestRecordPayment_NilIdempotencyKey_AllowsDuplicates(t *testing.T) {
	svc, _, paymentRepo, _, _, _ := newTestService()
	ctx := context.Background()
	orgID := uuid.New()
	userID := uuid.New()

	req := fin.CreatePaymentRequest{
		UnitID:      uuid.New(),
		AmountCents: 5000,
	}

	first, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)

	second, err := svc.RecordPayment(ctx, orgID, userID, req)
	require.NoError(t, err)

	assert.NotEqual(t, first.ID, second.ID, "nil key should create separate payments")
	assert.Len(t, paymentRepo.payments, 2)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestRecordPayment_NilIdempotencyKey -short -count=1`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add backend/internal/fin/service_test.go
git commit -m "test(fin): verify nil idempotency key allows duplicate payments (issue #80)"
```

---

## Part 3: Wire ComplianceResolver (Issue #84)

### Task 8: Wire ComplianceResolver into late fee calculation

**Files:**
- Modify: `backend/internal/fin/service.go:318-329`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCreateAssessment_ComplianceCapsLateFee(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	factory := wildcardTestFactory()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	// Use a compliance resolver that returns a fine_limits cap of $25.
	compliance := &mockComplianceResolver{
		rules: map[string][]ai.RuleValue{
			"fine_limits": {
				{Key: "max_late_fee_cents", ValueType: "integer", Value: json.RawMessage(`2500`)},
			},
		},
	}

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, nil, factory, ai.NewNoopPolicyResolver(), compliance, nil, logger, nil)
	ctx := context.Background()
	orgID := uuid.New()
	dueDate := time.Now().Add(30 * 24 * time.Hour)
	highFee := int64(10000) // $100 late fee

	req := fin.CreateAssessmentRequest{
		UnitID:       uuid.New(),
		Description:  "Q1 dues",
		AmountCents:  50000,
		DueDate:      dueDate,
		LateFeeCents: &highFee,
	}

	result, err := svc.CreateAssessment(ctx, orgID, req)
	require.NoError(t, err)
	require.NotNil(t, result.LateFeeCents)
	assert.Equal(t, int64(2500), *result.LateFeeCents, "late fee should be capped at compliance limit")
}
```

- [ ] **Step 2: Add the `mockComplianceResolver` test helper**

In `backend/internal/fin/service_test.go`:

```go
type mockComplianceResolver struct {
	rules map[string][]ai.RuleValue // category -> rules
}

func (m *mockComplianceResolver) GetJurisdictionRule(_ context.Context, _, category, key string) (*ai.RuleValue, error) {
	for _, r := range m.rules[category] {
		if r.Key == key {
			return &r, nil
		}
	}
	return nil, nil
}

func (m *mockComplianceResolver) ListJurisdictionRules(_ context.Context, _, category string) ([]ai.RuleValue, error) {
	return m.rules[category], nil
}

func (m *mockComplianceResolver) EvaluateCompliance(_ context.Context, _ uuid.UUID) (*ai.ComplianceReport, error) {
	return nil, nil
}

func (m *mockComplianceResolver) CheckCompliance(_ context.Context, _ uuid.UUID, category string) (*ai.ComplianceResult, error) {
	rules := m.rules[category]
	if len(rules) == 0 {
		return nil, nil
	}
	return &ai.ComplianceResult{
		Category:  category,
		Status:    "checked",
		Rules:     rules,
		CheckedAt: time.Now(),
	}, nil
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestCreateAssessment_ComplianceCapsLateFee -short -count=1`
Expected: FAIL — the late fee is not capped because `s.compliance` is never called.

- [ ] **Step 4: Implement compliance check in `CreateAssessment`**

In `backend/internal/fin/service.go`, after the existing late fee policy lookup (around line 329), add:

```go
	// Compliance: cap late fee at jurisdiction limit if available.
	if s.compliance != nil && a.LateFeeCents != nil {
		result, err := s.compliance.CheckCompliance(ctx, orgID, "fine_limits")
		if err == nil && result != nil {
			for _, rule := range result.Rules {
				if rule.Key == "max_late_fee_cents" {
					var cap int64
					if jsonErr := json.Unmarshal(rule.Value, &cap); jsonErr == nil && cap > 0 && *a.LateFeeCents > cap {
						a.LateFeeCents = &cap
					}
				}
			}
		}
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestCreateAssessment_ComplianceCapsLateFee -short -count=1`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): cap late fees via ComplianceResolver fine_limits (issue #84)"
```

### Task 9: Wire ComplianceResolver into fund transfer authorization

**Files:**
- Modify: `backend/internal/fin/service.go:1319-1327`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestCreateFundTransfer_ComplianceBlocksReserveWithdrawal(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	factory := wildcardTestFactory()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	compliance := &mockComplianceResolver{
		rules: map[string][]ai.RuleValue{
			"reserve_study": {
				{Key: "min_reserve_balance_cents", ValueType: "integer", Value: json.RawMessage(`100000`)},
			},
		},
	}

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, nil, factory, ai.NewNoopPolicyResolver(), compliance, nil, logger, nil)
	ctx := context.Background()
	orgID := uuid.New()

	// Create a reserve fund with 150000 balance.
	fromFundID := uuid.New()
	toFundID := uuid.New()
	funds.funds = []fin.Fund{
		{ID: fromFundID, OrgID: orgID, Name: "Reserve", FundType: fin.FundTypeReserve, BalanceCents: 150000},
		{ID: toFundID, OrgID: orgID, Name: "Operating", FundType: fin.FundTypeOperating, BalanceCents: 50000},
	}

	// Transfer 60000 from reserve — would leave 90000, below the 100000 minimum.
	req := fin.CreateFundTransferRequest{
		FromFundID:  fromFundID,
		ToFundID:    toFundID,
		AmountCents: 60000,
		Description: "Emergency transfer",
	}

	_, err := svc.CreateFundTransfer(ctx, orgID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reserve_study")
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestCreateFundTransfer_ComplianceBlocksReserveWithdrawal -short -count=1`
Expected: FAIL — the transfer succeeds because compliance is not checked.

- [ ] **Step 3: Implement compliance check in `CreateFundTransfer`**

In `backend/internal/fin/service.go`, after the balance sufficiency check (line 1326) and before `funds.CreateTransfer` (line 1328), add:

```go
	// Compliance: enforce reserve fund withdrawal minimums if applicable.
	if s.compliance != nil && fromFund.FundType == FundTypeReserve {
		result, compErr := s.compliance.CheckCompliance(ctx, orgID, "reserve_study")
		if compErr == nil && result != nil {
			for _, rule := range result.Rules {
				if rule.Key == "min_reserve_balance_cents" {
					var minBalance int64
					if jsonErr := json.Unmarshal(rule.Value, &minBalance); jsonErr == nil && minBalance > 0 {
						balanceAfter := fromFund.BalanceCents - req.AmountCents
						if balanceAfter < minBalance {
							return nil, api.NewValidationError(
								"fin.fund_transfer.compliance_reserve_minimum", "amount_cents",
								api.P("min_balance", minBalance),
								api.P("balance_after", balanceAfter),
								api.P("rule", "reserve_study"),
							)
						}
					}
				}
			}
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestCreateFundTransfer_ComplianceBlocksReserveWithdrawal -short -count=1`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): enforce reserve minimum via ComplianceResolver on fund transfers (issue #84)"
```

### Task 10: Wire ComplianceResolver into collection actions

**Files:**
- Modify: `backend/internal/fin/service.go:1405-1420`
- Modify: `backend/internal/fin/service_test.go`

- [ ] **Step 1: Write the failing test**

```go
func TestAddCollectionAction_ComplianceBlocksLienWithoutNotice(t *testing.T) {
	assessments := &mockAssessmentRepo{}
	payments := &mockPaymentRepo{}
	budgets := &mockBudgetRepo{}
	funds := &mockFundRepo{}
	collections := &mockCollectionRepo{}
	factory := wildcardTestFactory()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	compliance := &mockComplianceResolver{
		rules: map[string][]ai.RuleValue{
			"fine_limits": {
				{Key: "require_notice_before_lien", ValueType: "boolean", Value: json.RawMessage(`true`)},
			},
		},
	}

	svc := fin.NewFinService(assessments, payments, budgets, funds, collections, nil, factory, ai.NewNoopPolicyResolver(), compliance, nil, logger, nil)
	ctx := context.Background()

	// Create a collection case with no prior actions.
	caseID := uuid.New()
	collections.cases = []fin.CollectionCase{
		{ID: caseID, OrgID: uuid.New(), UnitID: uuid.New(), Status: fin.CollectionCaseStatusLate},
	}

	// Attempt to file a lien without a prior notice.
	req := fin.CreateCollectionActionRequest{
		ActionType: string(fin.CollectionActionTypeLienFiled),
		Notes:      strPtr("Filing lien"),
	}

	_, err := svc.AddCollectionAction(ctx, caseID, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "notice_required")
}
```

Also add a helper if not present:

```go
func strPtr(s string) *string { return &s }
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/fin/... -run TestAddCollectionAction_ComplianceBlocksLienWithoutNotice -short -count=1`
Expected: FAIL — lien action is created without compliance check.

- [ ] **Step 3: Implement compliance check in `AddCollectionAction`**

In `backend/internal/fin/service.go`, in `AddCollectionAction` after validation and before building the `CollectionAction` struct, add:

```go
	// Compliance: enforce notice-before-lien requirement per jurisdiction rules.
	if s.compliance != nil && CollectionActionType(req.ActionType) == CollectionActionTypeLienFiled {
		// Look up the case to get its org context.
		caseRecord, caseErr := s.collections.FindCaseByID(ctx, caseID)
		if caseErr != nil {
			return nil, fmt.Errorf("fin: AddCollectionAction case lookup: %w", caseErr)
		}
		if caseRecord != nil {
			result, compErr := s.compliance.CheckCompliance(ctx, caseRecord.OrgID, "fine_limits")
			if compErr == nil && result != nil {
				for _, rule := range result.Rules {
					if rule.Key == "require_notice_before_lien" {
						var required bool
						if jsonErr := json.Unmarshal(rule.Value, &required); jsonErr == nil && required {
							// Check if a notice has been sent for this case.
							actions, listErr := s.collections.ListActionsByCase(ctx, caseID)
							if listErr != nil {
								return nil, fmt.Errorf("fin: AddCollectionAction list actions: %w", listErr)
							}
							hasNotice := false
							for _, act := range actions {
								if act.ActionType == CollectionActionTypeNoticeSent {
									hasNotice = true
									break
								}
							}
							if !hasNotice {
								return nil, api.NewValidationError(
									"fin.collection.notice_required_before_lien", "action_type",
									api.P("rule", "fine_limits.require_notice_before_lien"),
								)
							}
						}
					}
				}
			}
		}
	}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd backend && go test ./internal/fin/... -run TestAddCollectionAction_ComplianceBlocksLienWithoutNotice -short -count=1`
Expected: PASS

- [ ] **Step 5: Run all tests to confirm nothing broke**

Run: `cd backend && go test ./internal/fin/... -short -count=1`
Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/fin/service.go backend/internal/fin/service_test.go
git commit -m "feat(fin): enforce notice-before-lien via ComplianceResolver (issue #84)"
```

### Task 11: Run lint and full test suite

- [ ] **Step 1: Run lint**

Run: `make lint`
Expected: no errors.

- [ ] **Step 2: Run full unit test suite**

Run: `make test`
Expected: all pass.

- [ ] **Step 3: Fix any issues and commit**

If lint or tests flag issues, fix and commit:

```bash
git commit -m "fix(fin): address lint/test issues from enum, idempotency, and compliance changes"
```
