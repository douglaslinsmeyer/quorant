package webhook

import (
	"context"

	"github.com/google/uuid"
)

// WebhookRepository defines persistence operations for the webhook domain.
type WebhookRepository interface {
	// Subscriptions
	CreateSubscription(ctx context.Context, s *Subscription) (*Subscription, error)
	FindSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error)
	ListSubscriptionsByOrg(ctx context.Context, orgID uuid.UUID) ([]Subscription, error)
	UpdateSubscription(ctx context.Context, s *Subscription) (*Subscription, error)
	SoftDeleteSubscription(ctx context.Context, id uuid.UUID) error

	// FindActiveSubscriptionsForEvent returns all active, non-deleted subscriptions
	// for the given org whose event_patterns match the given event type.
	// Patterns may contain wildcards (e.g. "quorant.gov.*").
	FindActiveSubscriptionsForEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Subscription, error)

	// Deliveries
	CreateDelivery(ctx context.Context, d *Delivery) (*Delivery, error)
	UpdateDelivery(ctx context.Context, d *Delivery) (*Delivery, error)
	ListDeliveriesBySubscription(ctx context.Context, subscriptionID uuid.UUID) ([]Delivery, error)
	FindPendingDeliveries(ctx context.Context) ([]Delivery, error) // for retry worker
}
