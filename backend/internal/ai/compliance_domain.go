package ai

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// JurisdictionRule represents a platform-managed statutory parameter.
type JurisdictionRule struct {
	ID               uuid.UUID       `json:"id"`
	Jurisdiction     string          `json:"jurisdiction"`
	RuleCategory     string          `json:"rule_category"`
	RuleKey          string          `json:"rule_key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    time.Time       `json:"effective_date"`
	ExpirationDate   *time.Time      `json:"expiration_date,omitempty"`
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      *uuid.UUID      `json:"source_doc_id,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	CreatedBy        *uuid.UUID      `json:"created_by,omitempty"`
}

// RuleValue is the read-model returned by ComplianceResolver lookups.
type RuleValue struct {
	ID               uuid.UUID       `json:"id"`
	Jurisdiction     string          `json:"jurisdiction"`
	Category         string          `json:"category"`
	Key              string          `json:"key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    time.Time       `json:"effective_date"`
	ExpirationDate   *time.Time      `json:"expiration_date,omitempty"`
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      *uuid.UUID      `json:"source_doc_id,omitempty"`
}

// IntValue unmarshals the JSONB value as an integer.
func (r *RuleValue) IntValue() (int, error) {
	var v int
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return 0, fmt.Errorf("rule %s/%s: expected integer: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// BoolValue unmarshals the JSONB value as a boolean.
func (r *RuleValue) BoolValue() (bool, error) {
	var v bool
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return false, fmt.Errorf("rule %s/%s: expected boolean: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// DecimalValue unmarshals the JSONB value as a float64.
func (r *RuleValue) DecimalValue() (float64, error) {
	var v float64
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return 0, fmt.Errorf("rule %s/%s: expected decimal: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// TextValue unmarshals the JSONB value as a string.
func (r *RuleValue) TextValue() (string, error) {
	var v string
	if err := json.Unmarshal(r.Value, &v); err != nil {
		return "", fmt.Errorf("rule %s/%s: expected text: %w", r.Category, r.Key, err)
	}
	return v, nil
}

// RuleValueFromJurisdictionRule converts a JurisdictionRule to a RuleValue.
func RuleValueFromJurisdictionRule(r *JurisdictionRule) RuleValue {
	return RuleValue{
		ID:               r.ID,
		Jurisdiction:     r.Jurisdiction,
		Category:         r.RuleCategory,
		Key:              r.RuleKey,
		ValueType:        r.ValueType,
		Value:            r.Value,
		StatuteReference: r.StatuteReference,
		EffectiveDate:    r.EffectiveDate,
		ExpirationDate:   r.ExpirationDate,
		Notes:            r.Notes,
		SourceDocID:      r.SourceDocID,
	}
}

// ComplianceCheck records a single compliance evaluation result for an org+rule.
type ComplianceCheck struct {
	ID              uuid.UUID       `json:"id"`
	OrgID           uuid.UUID       `json:"org_id"`
	RuleID          uuid.UUID       `json:"rule_id"`
	Status          string          `json:"status"`
	Details         json.RawMessage `json:"details,omitempty"`
	CheckedAt       time.Time       `json:"checked_at"`
	ResolvedAt      *time.Time      `json:"resolved_at,omitempty"`
	ResolutionNotes string          `json:"resolution_notes,omitempty"`
}

// ComplianceResult is the outcome of evaluating one category for an org.
type ComplianceResult struct {
	Category  string         `json:"category"`
	Status    string         `json:"status"`
	Rules     []RuleValue    `json:"rules"`
	Details   map[string]any `json:"details,omitempty"`
	CheckedAt time.Time      `json:"checked_at"`
}

// ComplianceReport is the full compliance evaluation for an org across all categories.
type ComplianceReport struct {
	OrgID        uuid.UUID          `json:"org_id"`
	Jurisdiction string             `json:"jurisdiction"`
	Results      []ComplianceResult `json:"results"`
	Summary      ComplianceSummary  `json:"summary"`
	CheckedAt    time.Time          `json:"checked_at"`
}

// ComplianceSummary summarizes a ComplianceReport.
type ComplianceSummary struct {
	Total         int `json:"total"`
	Compliant     int `json:"compliant"`
	NonCompliant  int `json:"non_compliant"`
	NotApplicable int `json:"not_applicable"`
	Unknown       int `json:"unknown"`
}
