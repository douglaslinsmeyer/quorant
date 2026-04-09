package ai

import "encoding/json"

// RegisterGoverningDocRequest is the request type for registering a governing document.
type RegisterGoverningDocRequest struct {
	DocumentID    string `json:"document_id"`    // required; UUID string
	DocType       string `json:"doc_type"`       // required: ccr, bylaws, rules, amendment, state_statute
	Title         string `json:"title"`          // required
	EffectiveDate string `json:"effective_date"` // required; YYYY-MM-DD
}

// ModifyExtractionRequest is the request type for applying a human override to an extraction.
type ModifyExtractionRequest struct {
	Override json.RawMessage `json:"override"` // required
}

// DecideResolutionRequest is the request type for recording a human decision on an escalated resolution.
type DecideResolutionRequest struct {
	Decision json.RawMessage `json:"decision"` // required
}
