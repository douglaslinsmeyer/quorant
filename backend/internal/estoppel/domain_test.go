package estoppel_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/estoppel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TestEstoppelRequest_ValidStatusTransitions
// ---------------------------------------------------------------------------

func TestEstoppelRequest_ValidStatusTransitions(t *testing.T) {
	tests := []struct {
		name  string
		from  string
		to    string
		valid bool
	}{
		// valid forward transitions
		{name: "submitted to data_aggregation", from: "submitted", to: "data_aggregation", valid: true},
		{name: "submitted to cancelled", from: "submitted", to: "cancelled", valid: true},
		{name: "data_aggregation to manager_review", from: "data_aggregation", to: "manager_review", valid: true},
		{name: "data_aggregation to cancelled", from: "data_aggregation", to: "cancelled", valid: true},
		{name: "manager_review to approved", from: "manager_review", to: "approved", valid: true},
		{name: "manager_review to cancelled", from: "manager_review", to: "cancelled", valid: true},
		{name: "approved to generating", from: "approved", to: "generating", valid: true},
		{name: "generating to delivered", from: "generating", to: "delivered", valid: true},
		// invalid transitions
		{name: "submitted to approved (skip)", from: "submitted", to: "approved", valid: false},
		{name: "delivered to generating (backward)", from: "delivered", to: "generating", valid: false},
		{name: "cancelled to submitted (reopen)", from: "cancelled", to: "submitted", valid: false},
		{name: "delivered to cancelled (terminal)", from: "delivered", to: "cancelled", valid: false},
		{name: "approved to manager_review (backward)", from: "approved", to: "manager_review", valid: false},
		{name: "generating to submitted (rewind)", from: "generating", to: "submitted", valid: false},
		{name: "unknown from state", from: "bogus", to: "submitted", valid: false},
		{name: "unknown to state", from: "submitted", to: "bogus", valid: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := estoppel.IsValidTransition(tc.from, tc.to)
			assert.Equal(t, tc.valid, result, "IsValidTransition(%q, %q)", tc.from, tc.to)
		})
	}
}

// ---------------------------------------------------------------------------
// TestCalculateFees_Standard
// ---------------------------------------------------------------------------

func TestCalculateFees_Standard(t *testing.T) {
	rules := &estoppel.EstoppelRules{
		StandardFeeCents:           25000,
		RushFeeCents:               15000,
		DelinquentSurchargeCents:   5000,
	}

	breakdown := estoppel.CalculateFees(rules, false, false)

	assert.Equal(t, int64(25000), breakdown.FeeCents, "FeeCents should be standard fee")
	assert.Equal(t, int64(0), breakdown.RushFeeCents, "RushFeeCents should be 0 for non-rush")
	assert.Equal(t, int64(0), breakdown.DelinquentSurchargeCents, "DelinquentSurchargeCents should be 0 for non-delinquent")
	assert.Equal(t, int64(25000), breakdown.TotalFeeCents, "TotalFeeCents should equal base fee only")
}

// ---------------------------------------------------------------------------
// TestCalculateFees_RushAndDelinquent
// ---------------------------------------------------------------------------

func TestCalculateFees_RushAndDelinquent(t *testing.T) {
	rules := &estoppel.EstoppelRules{
		StandardFeeCents:           25000,
		RushFeeCents:               15000,
		DelinquentSurchargeCents:   5000,
	}

	breakdown := estoppel.CalculateFees(rules, true, true)

	assert.Equal(t, int64(25000), breakdown.FeeCents, "FeeCents should be standard fee")
	assert.Equal(t, int64(15000), breakdown.RushFeeCents, "RushFeeCents should be rush fee")
	assert.Equal(t, int64(5000), breakdown.DelinquentSurchargeCents, "DelinquentSurchargeCents should be delinquent surcharge")
	assert.Equal(t, int64(45000), breakdown.TotalFeeCents, "TotalFeeCents should sum all fees")
}

// ---------------------------------------------------------------------------
// TestCalculateDeadline_Standard
// ---------------------------------------------------------------------------

func TestCalculateDeadline_Standard(t *testing.T) {
	rules := &estoppel.EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
	}

	// Use a Monday so we can predict the deadline more easily.
	// Monday 2026-04-06, adding 10 business days = Monday 2026-04-20
	from := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday

	deadline := estoppel.CalculateDeadline(rules, false, from)

	// 10 business days from Monday 2026-04-06:
	// Week 1: Tue Apr7, Wed Apr8, Thu Apr9, Fri Apr10 (4 days, total 4)
	// Week 2: Mon Apr13, Tue Apr14, Wed Apr15, Thu Apr16, Fri Apr17 (5 days, total 9)
	// Week 3: Mon Apr20 (1 day, total 10)
	expected := time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, deadline, "standard 10-business-day deadline from Monday")
}

// ---------------------------------------------------------------------------
// TestCalculateDeadline_Rush
// ---------------------------------------------------------------------------

func TestCalculateDeadline_Rush(t *testing.T) {
	rushDays := 3
	rules := &estoppel.EstoppelRules{
		StandardTurnaroundBusinessDays: 10,
		RushTurnaroundBusinessDays:     &rushDays,
	}

	// Monday 2026-04-06, adding 3 business days = Thursday 2026-04-09
	from := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday

	deadline := estoppel.CalculateDeadline(rules, true, from)

	// 3 business days from Monday Apr6: Tue Apr7, Wed Apr8, Thu Apr9
	expected := time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, deadline, "rush 3-business-day deadline from Monday")
}

// ---------------------------------------------------------------------------
// TestEstoppelRequest_JSONSerialization
// ---------------------------------------------------------------------------

func TestEstoppelRequest_JSONSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	closing := now.AddDate(0, 0, 30)
	deadline := now.AddDate(0, 0, 10)

	req := estoppel.EstoppelRequest{
		ID:               uuid.MustParse("00000000-0000-0000-0000-000000000001"),
		OrgID:            uuid.MustParse("00000000-0000-0000-0000-000000000002"),
		UnitID:           uuid.MustParse("00000000-0000-0000-0000-000000000003"),
		RequestType:      "estoppel_certificate",
		RequestorType:    "homeowner",
		RequestorName:    "Jane Smith",
		RequestorEmail:   "jane@example.com",
		PropertyAddress:  "123 Oak Street",
		OwnerName:        "Jane Smith",
		ClosingDate:      &closing,
		RushRequested:    false,
		Status:           "submitted",
		FeeCents:         25000,
		TotalFeeCents:    25000,
		DeadlineAt:       deadline,
		Metadata:         map[string]any{},
		CreatedBy:        uuid.MustParse("00000000-0000-0000-0000-000000000004"),
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	data, err := json.Marshal(req)
	require.NoError(t, err, "json.Marshal should not error")

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result), "json.Unmarshal should not error")

	requiredKeys := []string{
		"id", "org_id", "unit_id", "request_type", "requestor_type",
		"requestor_name", "requestor_email", "property_address", "owner_name",
		"closing_date", "rush_requested", "status",
		"fee_cents", "total_fee_cents", "deadline_at",
		"metadata", "created_by", "created_at", "updated_at",
	}
	for _, key := range requiredKeys {
		assert.Contains(t, result, key, "expected JSON key %q to be present", key)
	}

	// Optional nil fields should be omitted
	omittedKeys := []string{"task_id", "deleted_at", "assigned_to", "amendment_of"}
	for _, key := range omittedKeys {
		assert.NotContains(t, result, key, "expected JSON key %q to be omitted when nil", key)
	}
}
