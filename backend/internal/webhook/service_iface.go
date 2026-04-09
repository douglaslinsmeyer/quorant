package webhook

import (
	"context"

	"github.com/google/uuid"
)

// Service defines the business operations for the webhook module.
// Handlers depend on this interface rather than the concrete WebhookService struct.
type Service interface {
	// Subscriptions
	CreateSubscription(ctx context.Context, orgID, createdBy uuid.UUID, req CreateSubscriptionRequest) (*Subscription, error)
	ListSubscriptions(ctx context.Context, orgID uuid.UUID) ([]Subscription, error)
	GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error)
	UpdateSubscription(ctx context.Context, id uuid.UUID, req UpdateSubscriptionRequest) (*Subscription, error)
	DeleteSubscription(ctx context.Context, id uuid.UUID) error

	// Deliveries
	ListDeliveries(ctx context.Context, subscriptionID uuid.UUID) ([]Delivery, error)

	// Event Types
	GetEventTypes(_ context.Context) []string

	// Testing
	SendTestEvent(ctx context.Context, subscriptionID uuid.UUID) (*Delivery, error)
}
