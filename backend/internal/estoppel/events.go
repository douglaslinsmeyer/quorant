package estoppel

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/quorant/quorant/internal/platform/queue"
)

// ---------------------------------------------------------------------------
// Event type constants
// ---------------------------------------------------------------------------

const (
	// EventRequestCreated is published when a new estoppel request is submitted.
	EventRequestCreated = "estoppel_request.created"

	// EventDataAggregated is published when all provider data has been gathered
	// for an estoppel request.
	EventDataAggregated = "estoppel_request.data_aggregated"

	// EventRequestApproved is published when a manager approves an estoppel
	// request for document generation.
	EventRequestApproved = "estoppel_request.approved"

	// EventRequestRejected is published when a manager rejects an estoppel
	// request.
	EventRequestRejected = "estoppel_request.rejected"

	// EventCertificateGenerated is published when the PDF certificate has been
	// successfully generated.
	EventCertificateGenerated = "estoppel_request.certificate_generated"

	// EventCertificateDelivered is published when the certificate has been
	// delivered to the requestor.
	EventCertificateDelivered = "estoppel_request.delivered"

	// EventRequestCancelled is published when an estoppel request is cancelled.
	EventRequestCancelled = "estoppel_request.cancelled"

	// EventCertificateAmended is published when an amendment request is created
	// to correct a previously issued certificate.
	EventCertificateAmended = "estoppel_certificate.amended"
)

// aggregateType is the aggregate type label used for all estoppel events.
const aggregateType = "estoppel_request"

// newEstoppelEvent creates a BaseEvent for the estoppel module.
// payload may be any JSON-serialisable value; a nil payload encodes as "null".
func newEstoppelEvent(eventType string, requestID, orgID uuid.UUID, payload any) queue.BaseEvent {
	var raw json.RawMessage
	if payload != nil {
		if b, err := json.Marshal(payload); err == nil {
			raw = b
		}
	}
	return queue.NewBaseEvent(eventType, aggregateType, requestID, orgID, raw)
}
