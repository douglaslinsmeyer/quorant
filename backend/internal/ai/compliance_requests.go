package ai

import (
	"encoding/json"
	"fmt"
	"time"
)

// CreateJurisdictionRuleRequest is the DTO for POST /admin/jurisdiction-rules.
type CreateJurisdictionRuleRequest struct {
	Jurisdiction     string          `json:"jurisdiction"`
	RuleCategory     string          `json:"rule_category"`
	RuleKey          string          `json:"rule_key"`
	ValueType        string          `json:"value_type"`
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    string          `json:"effective_date"`
	Notes            string          `json:"notes,omitempty"`
	SourceDocID      string          `json:"source_doc_id,omitempty"`
}

// Validate returns an error if the request is missing required fields or contains invalid values.
func (r CreateJurisdictionRuleRequest) Validate() error {
	if r.Jurisdiction == "" {
		return fmt.Errorf("jurisdiction is required")
	}
	if !IsValidRuleCategory(r.RuleCategory) {
		return fmt.Errorf("invalid rule_category: %s", r.RuleCategory)
	}
	if r.RuleKey == "" {
		return fmt.Errorf("rule_key is required")
	}
	if !IsValidValueType(r.ValueType) {
		return fmt.Errorf("invalid value_type: %s", r.ValueType)
	}
	if len(r.Value) == 0 {
		return fmt.Errorf("value is required")
	}
	if r.StatuteReference == "" {
		return fmt.Errorf("statute_reference is required")
	}
	if r.EffectiveDate == "" {
		return fmt.Errorf("effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", r.EffectiveDate); err != nil {
		return fmt.Errorf("effective_date must be YYYY-MM-DD: %w", err)
	}
	return nil
}

// UpdateJurisdictionRuleRequest is the DTO for PUT /admin/jurisdiction-rules/{id}.
// Updating a rule expires the existing record and creates a new one with the changed values.
type UpdateJurisdictionRuleRequest struct {
	Value            json.RawMessage `json:"value"`
	StatuteReference string          `json:"statute_reference"`
	EffectiveDate    string          `json:"effective_date"`
	Notes            string          `json:"notes,omitempty"`
}

// Validate returns an error if the request is missing required fields or contains invalid values.
func (r UpdateJurisdictionRuleRequest) Validate() error {
	if len(r.Value) == 0 {
		return fmt.Errorf("value is required")
	}
	if r.StatuteReference == "" {
		return fmt.Errorf("statute_reference is required")
	}
	if r.EffectiveDate == "" {
		return fmt.Errorf("effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", r.EffectiveDate); err != nil {
		return fmt.Errorf("effective_date must be YYYY-MM-DD: %w", err)
	}
	return nil
}
