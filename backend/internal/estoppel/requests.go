package estoppel

import (
	"time"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/api"
)

// ---------------------------------------------------------------------------
// Enum sets
// ---------------------------------------------------------------------------

// validRequestTypes lists the accepted values for CreateEstoppelRequestDTO.RequestType.
var validRequestTypes = map[string]struct{}{
	"estoppel_certificate": {},
	"lender_questionnaire": {},
}

// validRequestorTypes lists the accepted values for CreateEstoppelRequestDTO.RequestorType.
var validRequestorTypes = map[string]struct{}{
	"homeowner":      {},
	"title_company":  {},
	"closing_agent":  {},
	"attorney":       {},
}

// ---------------------------------------------------------------------------
// CreateEstoppelRequestDTO
// ---------------------------------------------------------------------------

// CreateEstoppelRequestDTO carries the fields required to open a new estoppel
// or lender-questionnaire request.
type CreateEstoppelRequestDTO struct {
	UnitID           uuid.UUID  `json:"unit_id"`
	RequestType      string     `json:"request_type"`      // estoppel_certificate|lender_questionnaire
	RequestorType    string     `json:"requestor_type"`    // homeowner|title_company|closing_agent|attorney
	RequestorName    string     `json:"requestor_name"`
	RequestorEmail   string     `json:"requestor_email"`
	RequestorPhone   string     `json:"requestor_phone"`
	RequestorCompany string     `json:"requestor_company"`
	PropertyAddress  string     `json:"property_address"`
	OwnerName        string     `json:"owner_name"`
	ClosingDate      *time.Time `json:"closing_date,omitempty"`
	RushRequested    bool       `json:"rush_requested"`
	Metadata         map[string]any `json:"metadata,omitempty"`
}

// Validate returns a ValidationError for the first constraint violation found,
// or nil when the DTO is valid.
func (d *CreateEstoppelRequestDTO) Validate() error {
	if d.UnitID == uuid.Nil {
		return api.NewValidationError("unit_id is required", "unit_id")
	}
	if _, ok := validRequestTypes[d.RequestType]; !ok {
		return api.NewValidationError(
			"request_type must be one of: estoppel_certificate, lender_questionnaire",
			"request_type",
		)
	}
	if _, ok := validRequestorTypes[d.RequestorType]; !ok {
		return api.NewValidationError(
			"requestor_type must be one of: homeowner, title_company, closing_agent, attorney",
			"requestor_type",
		)
	}
	if d.RequestorName == "" {
		return api.NewValidationError("requestor_name is required", "requestor_name")
	}
	if d.RequestorEmail == "" {
		return api.NewValidationError("requestor_email is required", "requestor_email")
	}
	if d.PropertyAddress == "" {
		return api.NewValidationError("property_address is required", "property_address")
	}
	if d.OwnerName == "" {
		return api.NewValidationError("owner_name is required", "owner_name")
	}
	return nil
}

// ---------------------------------------------------------------------------
// ApproveRequestDTO
// ---------------------------------------------------------------------------

// ApproveRequestDTO carries the fields required to approve an estoppel request
// and advance it to the "generating" phase.
type ApproveRequestDTO struct {
	SignerTitle string `json:"signer_title"`
	Notes      string `json:"notes,omitempty"`
}

// Validate returns a ValidationError if SignerTitle is empty.
func (d *ApproveRequestDTO) Validate() error {
	if d.SignerTitle == "" {
		return api.NewValidationError("signer_title is required", "signer_title")
	}
	return nil
}

// ---------------------------------------------------------------------------
// RejectRequestDTO
// ---------------------------------------------------------------------------

// RejectRequestDTO carries the fields required to cancel/reject an estoppel
// request.
type RejectRequestDTO struct {
	Reason string `json:"reason"`
}

// Validate returns a ValidationError if Reason is empty.
func (d *RejectRequestDTO) Validate() error {
	if d.Reason == "" {
		return api.NewValidationError("reason is required", "reason")
	}
	return nil
}

// ---------------------------------------------------------------------------
// UpdateNarrativesDTO
// ---------------------------------------------------------------------------

// UpdateNarrativesDTO carries updated narrative sections for manager review.
// All fields in NarrativeSections may be edited before approval.
type UpdateNarrativesDTO struct {
	Narratives NarrativeSections `json:"narratives"`
}

// ---------------------------------------------------------------------------
// AmendCertificateDTO
// ---------------------------------------------------------------------------

// AmendCertificateDTO carries the optional reason for amending a previously
// issued estoppel certificate.
type AmendCertificateDTO struct {
	Reason string `json:"reason,omitempty"`
}
