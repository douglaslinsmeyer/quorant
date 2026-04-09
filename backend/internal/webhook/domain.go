package webhook

import (
	"time"

	"github.com/google/uuid"
)

// Subscription represents a webhook subscription for an organization.
type Subscription struct {
	ID            uuid.UUID         `json:"id"`
	OrgID         uuid.UUID         `json:"org_id"`
	Name          string            `json:"name"`
	EventPatterns []string          `json:"event_patterns"`
	TargetURL     string            `json:"target_url"`
	Secret        string            `json:"-"` // never expose in API responses
	Headers       map[string]string `json:"headers"`
	IsActive      bool              `json:"is_active"`
	RetryPolicy   RetryPolicy       `json:"retry_policy"`
	CreatedBy     uuid.UUID         `json:"created_by"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
	DeletedAt     *time.Time        `json:"deleted_at,omitempty"`
}

// RetryPolicy defines how delivery retries are scheduled.
type RetryPolicy struct {
	MaxRetries     int   `json:"max_retries"`
	BackoffSeconds []int `json:"backoff_seconds"`
}

// DefaultRetryPolicy returns the standard retry policy for new subscriptions.
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		MaxRetries:     3,
		BackoffSeconds: []int{5, 30, 300},
	}
}

// Delivery records a single attempt to deliver an event to a subscription.
type Delivery struct {
	ID             uuid.UUID  `json:"id"`
	SubscriptionID uuid.UUID  `json:"subscription_id"`
	EventID        uuid.UUID  `json:"event_id"`
	EventType      string     `json:"event_type"`
	Status         string     `json:"status"` // pending, delivered, failed, retrying
	Attempts       int        `json:"attempts"`
	LastAttemptAt  *time.Time `json:"last_attempt_at,omitempty"`
	ResponseCode   *int       `json:"response_code,omitempty"`
	ResponseBody   *string    `json:"response_body,omitempty"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Delivery status constants.
const (
	DeliveryStatusPending   = "pending"
	DeliveryStatusDelivered = "delivered"
	DeliveryStatusFailed    = "failed"
	DeliveryStatusRetrying  = "retrying"
)

// AvailableEventTypes returns the list of event types that can be subscribed to.
func AvailableEventTypes() []string {
	return []string{
		"quorant.org.MembershipChanged", "quorant.org.OwnershipTransferred",
		"quorant.org.FirmConnected", "quorant.org.FirmDisconnected",
		"quorant.fin.PaymentReceived", "quorant.fin.AssessmentDue",
		"quorant.fin.CollectionCaseOpened", "quorant.fin.CollectionEscalated",
		"quorant.gov.ViolationCreated", "quorant.gov.ViolationResolved",
		"quorant.gov.ARBRequestSubmitted", "quorant.gov.ARBAutoApproved",
		"quorant.gov.MeetingScheduled", "quorant.gov.MotionPassed",
		"quorant.com.AnnouncementPublished", "quorant.com.NotificationRequested",
		"quorant.doc.DocumentUploaded", "quorant.doc.GoverningDocUploaded",
		"quorant.task.TaskCreated", "quorant.task.TaskCompleted",
		"quorant.billing.PaymentFailed", "quorant.billing.SubscriptionCancelled",
		"quorant.ai.PolicyExtractionComplete", "quorant.ai.PolicyResolutionEscalated",
	}
}
