// Package estoppel provides domain types, interfaces, and business logic for
// the Estoppel module: estoppel certificates, lender questionnaires, fee
// calculation, deadline computation, and data aggregation.
package estoppel

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// EstoppelRequest is the intake record for an estoppel certificate or lender
// questionnaire request.
type EstoppelRequest struct {
	ID                      uuid.UUID      `json:"id"`
	OrgID                   uuid.UUID      `json:"org_id"`
	UnitID                  uuid.UUID      `json:"unit_id"`
	TaskID                  *uuid.UUID     `json:"task_id,omitempty"`
	RequestType             string         `json:"request_type"`             // estoppel_certificate | lender_questionnaire
	RequestorType           string         `json:"requestor_type"`           // homeowner | title_company | closing_agent | attorney
	RequestorName           string         `json:"requestor_name"`
	RequestorEmail          string         `json:"requestor_email"`
	RequestorPhone          string         `json:"requestor_phone"`
	RequestorCompany        string         `json:"requestor_company"`
	PropertyAddress         string         `json:"property_address"`
	OwnerName               string         `json:"owner_name"`
	ClosingDate             *time.Time     `json:"closing_date,omitempty"`
	RushRequested           bool           `json:"rush_requested"`
	Status                  string         `json:"status"` // submitted|data_aggregation|manager_review|approved|generating|delivered|cancelled
	FeeCents                int64          `json:"fee_cents"`
	RushFeeCents            int64          `json:"rush_fee_cents"`
	DelinquentSurchargeCents int64         `json:"delinquent_surcharge_cents"`
	TotalFeeCents           int64          `json:"total_fee_cents"`
	DeadlineAt              time.Time      `json:"deadline_at"`
	AssignedTo              *uuid.UUID     `json:"assigned_to,omitempty"`
	AmendmentOf             *uuid.UUID     `json:"amendment_of,omitempty"`
	Metadata                map[string]any `json:"metadata"`
	CreatedBy               uuid.UUID      `json:"created_by"`
	CreatedAt               time.Time      `json:"created_at"`
	UpdatedAt               time.Time      `json:"updated_at"`
	DeletedAt               *time.Time     `json:"deleted_at,omitempty"`
}

// EstoppelCertificate is the generated output document for an estoppel request.
type EstoppelCertificate struct {
	ID               uuid.UUID       `json:"id"`
	RequestID        uuid.UUID       `json:"request_id"`
	OrgID            uuid.UUID       `json:"org_id"`
	UnitID           uuid.UUID       `json:"unit_id"`
	DocumentID       *uuid.UUID      `json:"document_id,omitempty"`
	Jurisdiction     string          `json:"jurisdiction"`
	EffectiveDate    time.Time       `json:"effective_date"`
	ExpiresAt        *time.Time      `json:"expires_at,omitempty"`
	DataSnapshot     json.RawMessage `json:"data_snapshot"`
	NarrativeSections json.RawMessage `json:"narrative_sections"`
	SignedBy         uuid.UUID       `json:"signed_by"`
	SignedAt         time.Time       `json:"signed_at"`
	SignerTitle      string          `json:"signer_title"`
	TemplateVersion  string          `json:"template_version"`
	AmendmentOf      *uuid.UUID      `json:"amendment_of,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
}

// EstoppelRules holds the deserialized policy configuration for estoppel
// processing within an HOA organization.
type EstoppelRules struct {
	StandardTurnaroundBusinessDays int     `json:"standard_turnaround_business_days"`
	StandardFeeCents               int64   `json:"standard_fee_cents"`
	RushTurnaroundBusinessDays     *int    `json:"rush_turnaround_business_days,omitempty"`
	RushFeeCents                   int64   `json:"rush_fee_cents"`
	DelinquentSurchargeCents       int64   `json:"delinquent_surcharge_cents"`
	EffectivePeriodDays            *int    `json:"effective_period_days,omitempty"`
	ElectronicDeliveryRequired     bool    `json:"electronic_delivery_required"`
	StatutoryFormRequired          bool    `json:"statutory_form_required"`
	StatutoryFormID                string  `json:"statutory_form_id"`
	FreeAmendmentOnError           bool    `json:"free_amendment_on_error"`
	StatuteRef                     string  `json:"statute_ref"`
	RequiredAttachments            []string `json:"required_attachments"`
}

// FeeBreakdown holds the calculated fee components for an estoppel request.
type FeeBreakdown struct {
	FeeCents                 int64 `json:"fee_cents"`
	RushFeeCents             int64 `json:"rush_fee_cents"`
	DelinquentSurchargeCents int64 `json:"delinquent_surcharge_cents"`
	TotalFeeCents            int64 `json:"total_fee_cents"`
}

// validTransitions defines the allowed status transitions for EstoppelRequest.
// Terminal states (delivered, cancelled) have no outgoing transitions.
var validTransitions = map[string][]string{
	"submitted":        {"data_aggregation", "cancelled"},
	"data_aggregation": {"manager_review", "cancelled"},
	"manager_review":   {"approved", "cancelled"},
	"approved":         {"generating"},
	"generating":       {"delivered"},
	"delivered":        {},
	"cancelled":        {},
}

// IsValidTransition reports whether transitioning an EstoppelRequest from the
// given status to the target status is permitted.
func IsValidTransition(from, to string) bool {
	targets, ok := validTransitions[from]
	if !ok {
		return false
	}
	for _, t := range targets {
		if t == to {
			return true
		}
	}
	return false
}

// CalculateFees computes the fee breakdown for an estoppel request based on
// the organisation's rules and whether rush processing or a delinquency
// surcharge applies.
func CalculateFees(rules *EstoppelRules, rush, delinquent bool) FeeBreakdown {
	bd := FeeBreakdown{
		FeeCents: rules.StandardFeeCents,
	}
	if rush {
		bd.RushFeeCents = rules.RushFeeCents
	}
	if delinquent {
		bd.DelinquentSurchargeCents = rules.DelinquentSurchargeCents
	}
	bd.TotalFeeCents = bd.FeeCents + bd.RushFeeCents + bd.DelinquentSurchargeCents
	return bd
}

// CalculateDeadline returns the deadline time.Time for an estoppel request by
// adding the appropriate number of business days (Mon–Fri only) to from.
// If rush is true and RushTurnaroundBusinessDays is set, the rush turnaround
// is used; otherwise StandardTurnaroundBusinessDays is used.
func CalculateDeadline(rules *EstoppelRules, rush bool, from time.Time) time.Time {
	days := rules.StandardTurnaroundBusinessDays
	if rush && rules.RushTurnaroundBusinessDays != nil {
		days = *rules.RushTurnaroundBusinessDays
	}

	current := from
	added := 0
	for added < days {
		current = current.AddDate(0, 0, 1)
		wd := current.Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			added++
		}
	}
	return current
}
