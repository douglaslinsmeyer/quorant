package billing_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/billing"
)

// ─── BillingAccount JSON serialization ───────────────────────────────────────

func TestBillingAccount_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	a := billing.BillingAccount{
		ID:           uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		OrgID:        uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		BillingEmail: "billing@example.com",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal(BillingAccount) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"id", "org_id", "billing_email", "created_at", "updated_at"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestBillingAccount_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	a := billing.BillingAccount{
		ID:           uuid.New(),
		OrgID:        uuid.New(),
		BillingEmail: "billing@example.com",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"stripe_customer_id", "billing_name"} {
		if _, ok := result[key]; ok {
			t.Errorf("expected %q to be omitted when nil", key)
		}
	}
}

func TestBillingAccount_JSONSerialization_IncludesOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	stripeID := "cus_test123"
	name := "Acme HOA"
	a := billing.BillingAccount{
		ID:               uuid.New(),
		OrgID:            uuid.New(),
		StripeCustomerID: &stripeID,
		BillingEmail:     "billing@example.com",
		BillingName:      &name,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	if result["stripe_customer_id"] != stripeID {
		t.Errorf("stripe_customer_id: got %v, want %v", result["stripe_customer_id"], stripeID)
	}
	if result["billing_name"] != name {
		t.Errorf("billing_name: got %v, want %v", result["billing_name"], name)
	}
}

// ─── Invoice JSON serialization ───────────────────────────────────────────────

func TestInvoice_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	inv := billing.Invoice{
		ID:               uuid.New(),
		BillingAccountID: uuid.New(),
		OrgID:            uuid.New(),
		Status:           "draft",
		SubtotalCents:    5000,
		TaxCents:         500,
		TotalCents:       5500,
		PeriodStart:      now.Add(-30 * 24 * time.Hour),
		PeriodEnd:        now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("json.Marshal(Invoice) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{
		"id", "billing_account_id", "org_id", "status",
		"subtotal_cents", "tax_cents", "total_cents",
		"period_start", "period_end", "created_at", "updated_at",
	} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestInvoice_JSONSerialization_OmitsNilOptionalFields(t *testing.T) {
	now := time.Now().UTC()
	inv := billing.Invoice{
		ID:               uuid.New(),
		BillingAccountID: uuid.New(),
		OrgID:            uuid.New(),
		Status:           "draft",
		PeriodStart:      now.Add(-30 * 24 * time.Hour),
		PeriodEnd:        now,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{"stripe_invoice_id", "due_date", "paid_at"} {
		if _, ok := result[key]; ok {
			t.Errorf("expected %q to be omitted when nil", key)
		}
	}
}

// ─── InvoiceLineItem JSON serialization ──────────────────────────────────────

func TestInvoiceLineItem_JSONSerialization_RequiredFieldsPresent(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	item := billing.InvoiceLineItem{
		ID:             uuid.New(),
		InvoiceID:      uuid.New(),
		Description:    "Subscription - Starter Plan",
		Quantity:       1,
		UnitPriceCents: 4999,
		TotalCents:     4999,
		LineType:       "subscription",
		Metadata:       map[string]any{},
		CreatedAt:      now,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal(InvoiceLineItem) error: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	for _, key := range []string{
		"id", "invoice_id", "description", "quantity",
		"unit_price_cents", "total_cents", "line_type", "metadata", "created_at",
	} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected JSON key %q to be present", key)
		}
	}
}

func TestInvoiceLineItem_JSONSerialization_MetadataPreserved(t *testing.T) {
	now := time.Now().UTC()
	item := billing.InvoiceLineItem{
		ID:             uuid.New(),
		InvoiceID:      uuid.New(),
		Description:    "Overage",
		Quantity:       5,
		UnitPriceCents: 100,
		TotalCents:     500,
		LineType:       "overage",
		Metadata:       map[string]any{"feature": "api_calls", "period": "2026-03"},
		CreatedAt:      now,
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("json.Unmarshal error: %v", err)
	}

	meta, ok := result["metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected metadata to be an object, got %T", result["metadata"])
	}
	if meta["feature"] != "api_calls" {
		t.Errorf("metadata.feature: got %v, want api_calls", meta["feature"])
	}
}

// ─── CreateBillingAccountRequest.Validate() ──────────────────────────────────

func TestCreateBillingAccountRequest_Validate_ValidRequest(t *testing.T) {
	req := billing.CreateBillingAccountRequest{BillingEmail: "billing@acme.com"}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for valid request, got: %v", err)
	}
}

func TestCreateBillingAccountRequest_Validate_MissingEmailReturnsError(t *testing.T) {
	req := billing.CreateBillingAccountRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when billing_email is missing, got nil")
	}
}

func TestCreateBillingAccountRequest_Validate_WithOptionalName(t *testing.T) {
	name := "Acme HOA"
	req := billing.CreateBillingAccountRequest{
		BillingEmail: "billing@acme.com",
		BillingName:  &name,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error for request with optional name, got: %v", err)
	}
}

// ─── UpdateBillingAccountRequest.Validate() ──────────────────────────────────

func TestUpdateBillingAccountRequest_Validate_WithEmailOnly(t *testing.T) {
	email := "new@acme.com"
	req := billing.UpdateBillingAccountRequest{BillingEmail: &email}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with billing_email set, got: %v", err)
	}
}

func TestUpdateBillingAccountRequest_Validate_WithNameOnly(t *testing.T) {
	name := "Updated Name"
	req := billing.UpdateBillingAccountRequest{BillingName: &name}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with billing_name set, got: %v", err)
	}
}

func TestUpdateBillingAccountRequest_Validate_WithStripeIDOnly(t *testing.T) {
	id := "cus_abc123"
	req := billing.UpdateBillingAccountRequest{StripeCustomerID: &id}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with stripe_customer_id set, got: %v", err)
	}
}

func TestUpdateBillingAccountRequest_Validate_EmptyRequestReturnsError(t *testing.T) {
	req := billing.UpdateBillingAccountRequest{}
	if err := req.Validate(); err == nil {
		t.Error("expected error when no fields are provided, got nil")
	}
}

func TestUpdateBillingAccountRequest_Validate_AllFieldsSet(t *testing.T) {
	email := "new@acme.com"
	name := "New Name"
	stripeID := "cus_xyz"
	req := billing.UpdateBillingAccountRequest{
		BillingEmail:     &email,
		BillingName:      &name,
		StripeCustomerID: &stripeID,
	}
	if err := req.Validate(); err != nil {
		t.Errorf("expected nil error with all fields set, got: %v", err)
	}
}
